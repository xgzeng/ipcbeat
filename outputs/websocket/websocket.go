package websocket

import (
	"os"

	"html/template"
	"log"
	"net/http"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
	"github.com/elastic/beats/libbeat/outputs/codec"
	"github.com/elastic/beats/libbeat/outputs/codec/json"
	"github.com/elastic/beats/libbeat/publisher"
	"github.com/gorilla/websocket"
)

type wsConnection struct {
	chEvent chan []byte
}

func makeWsConnection(c *websocket.Conn, o *wsOutput) *wsConnection {
	wsc := &wsConnection{
		chEvent: make(chan []byte),
	}

	// reader routine
	go func() {
		defer c.Close()
		defer o.delWsConnection(wsc)

		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
		}
	}()

	// forward routine
	go func() {
		for {
			message := <-wsc.chEvent
			log.Printf("recv: %s", message)
			err := c.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}()

	return wsc
}

func (c *wsConnection) publish(message []byte) {
	log.Print("wsConnection::publish ", string(message))
	c.chEvent <- message
}

type wsOutput struct {
	observer   outputs.Observer
	codec      codec.Codec
	index      string
	httpServer *http.Server
	conns      []*wsConnection
	// internal channel
	chEvent     chan []byte
	chNewConn   chan *wsConnection
	chCloseConn chan *wsConnection
}

// Init websocket output module
func Init() {
	outputs.RegisterType("websocket", makeOutput)
}

var upgrader = websocket.Upgrader{}

func (o *wsOutput) delWsConnection(c *wsConnection) {
	o.chCloseConn <- c
}

func (o *wsOutput) getWsConnection(index int) *wsConnection {
	if index < len(o.conns) {
		return o.conns[index]
	}

	return nil
}

// handle incoming websocket url
func (o *wsOutput) handleWSConn(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	wsc := makeWsConnection(c, o)
	o.chNewConn <- wsc
}

func (o *wsOutput) run() {
	// run web server
	go func() {
		logp.Info("Serve http request on " + o.httpServer.Addr)
		err := o.httpServer.ListenAndServe()
		log.Fatal(err)
	}()

	// forward logs
	for {
		select {
		case c := <-o.chNewConn:
			o.conns = append(o.conns, c)
		case c := <-o.chCloseConn:
			// find and delete connection
			var i int
			for i = 0; i < len(o.conns); i++ {
				if c == o.conns[i] {
					break
				}
			}
			if i < len(o.conns) {
				o.conns = append(o.conns[:i], o.conns[i+1:]...)
			}
		case e := <-o.chEvent:
			// forward event for every connection
			for _, c := range o.conns {
				c.publish(e)
			}
		}
	}
}

type rootRedirectHandler struct {
	handler http.Handler
}

func (h *rootRedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path == "/" {
		// redirect to index.htm
		http.Redirect(w, r, "/index.htm", http.StatusFound)
	} else {
		h.handler.ServeHTTP(w, r)
	}
}

func (o *wsOutput) init(httpAddr string) {
	// init http server
	mux := http.NewServeMux()
	mux.HandleFunc("/log", o.handleWSConn)

	// check default htm static directory
	finfo, err := os.Stat("./html")
	if err == nil && finfo.IsDir() {
		rootHandler := &rootRedirectHandler{
			handler: http.FileServer(http.Dir("./html")),
		}

		mux.Handle("/", rootHandler)
	} else {
		homeTemplate, err := template.ParseFiles("home.template")
		if err != nil {
			homeTemplate = defaultHomeTemplate
		}
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			homeTemplate.Execute(w, "ws://"+r.Host+"/log")
		})
	}

	o.httpServer = &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}
}

func makeOutput(
	beat beat.Info,
	observer outputs.Observer,
	cfg *common.Config) (outputs.Group, error) {

	config := defaultConfig
	err := cfg.Unpack(&config)
	if err != nil {
		return outputs.Fail(err)
	}

	if config.Addr == "" {
		config.Addr = "localhost:8080"
	}

	enc := json.New(false /* no pretty */, beat.Version)
	index := beat.Beat

	o := &wsOutput{
		codec:       enc,
		observer:    observer,
		index:       index,
		chNewConn:   make(chan *wsConnection),
		chCloseConn: make(chan *wsConnection),
		chEvent:     make(chan []byte),
	}

	log.SetFlags(0)

	o.init(config.Addr)
	// output internal routine
	go o.run()

	return outputs.Success(config.BatchSize, 0, o)
}

func (o *wsOutput) Close() error {
	o.httpServer.Close()
	return nil
}

func (o *wsOutput) Publish(batch publisher.Batch) error {
	events := batch.Events()
	o.observer.NewBatch(len(events))

	dropped := 0
	for i := range events {
		ok := o.publishEvent(&events[i])
		if !ok {
			dropped++
		}
	}

	batch.ACK()

	o.observer.Dropped(dropped)
	o.observer.Acked(len(events) - dropped)

	return nil
}

func (o *wsOutput) publishEvent(event *publisher.Event) bool {
	serializedEvent, err := o.codec.Encode(o.index, &event.Content)
	if err != nil {
		if !event.Guaranteed() {
			return false
		}

		logp.Critical("Unable to encode event: %v", err)
		return false
	}

	o.chEvent <- serializedEvent

	// if err := o.writeBuffer(serializedEvent); err != nil {
	// 	o.observer.WriteError(err)
	// 	logp.Critical("Unable to publish events to console: %v", err)
	// 	return false
	// }
	// if err := o.writeBuffer(nl); err != nil {
	// 	o.observer.WriteError(err)
	// 	logp.Critical("Error when appending newline to event: %v", err)
	// 	return false
	// }

	return true
}

var defaultHomeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.innerHTML = message;
        output.appendChild(d);
    };

    function OpenWebSocket() {
        if (ws) {
            return false;
        }
				ws_url = "ws://" + window.location.host + "/log"
        ws = new WebSocket(ws_url)
        ws.onopen = function(evt) {
						print(ws_url + " connected, wait for log item")
            console.info("web socket opened")
        }
        ws.onclose = function(evt) {
            alert("web socket closed")
            console.error("web socket closed")
            ws = null;
        }

        ws.onmessage = function(evt) {
            print(evt.data);
        }

        ws.onerror = function(evt) {
            alert("web socket error: " + evt.data)
        }

        return false;
    }

    OpenWebSocket()

    function CloseWebSocket() {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    }
});
</script>
</head>
<body>
<div id="output"></div>
</body>
</html>
`))
