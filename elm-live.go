/*
 * [!] A quick hack to enable
 * "live-coding" in Elm
 *
 * [+] Forked
 *  "github.com/gorilla/websocket"
 * and serving elm-generated
 * javascript instead of general
 * files.
 *
 * [+] Basic idea: load
 * elm-generated javascript into
 * a script tag and call
 * `Elm.Main.embed` to refresh
 *
 * [+-] Main issue: The model
 * should not reset on each load
 *
 */
package main

import (
    "flag"
    "text/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "strconv"
    "time"
    "os/exec"
    "strings"

    "github.com/gorilla/websocket"
)

const (
    // Time allowed to write the file to the client.
    writeWait = 10 * time.Second

    // Time allowed to read the next pong message from the client.
    pongWait = 60 * time.Second

    // Send pings to client with this period. Must be less than pongWait.
    pingPeriod = (pongWait * 9) / 10

    // Poll file for changes with this period.
    filePeriod = 200 * time.Millisecond
)

var (
    addr      = flag.String("addr", ":8080", "http service address")
    homeTempl = template.Must(template.New("").Parse(homeHTML))
    filename  string
    upgrader  = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }
)

func outputFileName_1(n string) string {
    return n[:len(n)-len("elm")] + "js"
}

func compileElm(in, out string) string {
    cmd := exec.Command("elm-make", in, "--yes","--output=" + out)
    o,err := cmd.CombinedOutput()
    if err != nil {
        return string(o)
    }
    return ""
}

func readFileIfModified(lastMod time.Time) ([]byte, time.Time, error) {
    fi, err := os.Stat(filename)
    if err != nil {
        return nil, lastMod, err
    }
    if !fi.ModTime().After(lastMod) {
        return nil, lastMod, nil
    }

    outputjs := outputFileName_1(filename)

    e := compileElm(filename, outputjs)
    if len(e) != 0 {
        log.Println(e)
        return nil, fi.ModTime(), nil
    } else {
        log.Println("Compiled...reloading")
    }

    p, err := ioutil.ReadFile(outputjs)
    if err != nil {
        return nil, fi.ModTime(), err
    }
    return p, fi.ModTime(), nil
}

func reader(ws *websocket.Conn) {
    defer ws.Close()
    ws.SetReadLimit(512)
    ws.SetReadDeadline(time.Now().Add(pongWait))
    ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
    for {
        _, _, err := ws.ReadMessage()
        if err != nil {
            break
        }
    }
}

func writer(ws *websocket.Conn, lastMod time.Time) {
    lastError := ""
    pingTicker := time.NewTicker(pingPeriod)
    fileTicker := time.NewTicker(filePeriod)
    defer func() {
        pingTicker.Stop()
        fileTicker.Stop()
        ws.Close()
    }()
    for {
        select {
        case <-fileTicker.C:
            var p []byte
            var err error

            p, lastMod, err = readFileIfModified(lastMod)

            if err != nil {
                if s := err.Error(); s != lastError {
                    lastError = s
                    p = []byte(lastError)
                }
            } else {
                lastError = ""
            }

            if p != nil {
                ws.SetWriteDeadline(time.Now().Add(writeWait))
                if err := ws.WriteMessage(websocket.TextMessage, p); err != nil {
                    return
                }
            }
        case <-pingTicker.C:
            ws.SetWriteDeadline(time.Now().Add(writeWait))
            if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
                return
            }
        }
    }
}

func serveWs(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        if _, ok := err.(websocket.HandshakeError); !ok {
            log.Println(err)
        }
        return
    }

    var lastMod time.Time
    if n, err := strconv.ParseInt(r.FormValue("lastMod"), 16, 64); err == nil {
        lastMod = time.Unix(0, n)
    }

    go writer(ws, lastMod)
    reader(ws)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.Error(w, "Not found", 404)
        return
    }
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", 405)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    p, lastMod, err := readFileIfModified(time.Time{})
    if err != nil {
        p = []byte(err.Error())
        lastMod = time.Unix(0, 0)
    }
    var v = struct {
        Host    string
        Title   string
        Data    string
        LastMod string
    }{
        r.Host,
        filename,
        string(p),
        strconv.FormatInt(lastMod.UnixNano(), 16),
    }
    homeTempl.Execute(w, &v)
}

func isNotElm(n string) bool {
    return !strings.HasSuffix(n, ".elm")
}

func main() {
    flag.Parse()
    if flag.NArg() != 1 {
        log.Fatal("filename not specified")
    }
    filename = flag.Args()[0]
    if isNotElm(filename) {
        log.Fatal("Need an .elm program")
    }
    http.HandleFunc("/", serveHome)
    http.HandleFunc("/ws", serveWs)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatal(err)
    }
}

const homeHTML = `<!DOCTYPE html>
<html lang="en">
    <head>
        <title>{{.Title}}</title>
    </head>
    <body>
        <div id=elmnode></div>
        <div id=elmscript>
            <script>{{.Data}}</script>
        </div>
        <script type="text/javascript">
            var data = document.getElementById("elmnode");
            var scr = document.getElementById("elmscript");
            (function() {
                var conn = new WebSocket("ws://{{.Host}}/ws?lastMod={{.LastMod}}");
                conn.onclose = function(evt) {
                    data.textContent = 'Connection closed';
                }
                conn.onmessage = function(evt) {
                    console.log('file updated');
                    var s = document.createElement("script");
                    s.textContent = evt.data;

                    Elm = undefined;
                    scr.innerHTML = "";
                    scr.appendChild(s);

                    setTimeout(function() {
                        data.innerHTML = "";
                        Elm.Main.embed(data);
                    }, 10);
                }
            })();
            data.innerHTML = "";
            Elm.Main.embed(data);
        </script>
    </body>
</html>
`
