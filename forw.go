package main

import (
    "bytes"
    "fmt"
    "encoding/json"
    "io"
    "io/ioutil"
    "os"
    "os/signal"
    "syscall"
    "flag"
    "net"
    "net/http"
    "net/http/httputil"
    "time"
)

var (
    file = flag.String("f", "config.json", "Path to config file")
    debug = flag.Bool("d", false, "Debug messages")
)

type nopCloser struct {
    io.Reader
}

func (nopCloser) Close() error { return nil }

func DuplicateRequest(request *http.Request) (request1 *http.Request, request2 *http.Request) {
    b1 := new(bytes.Buffer)
    b2 := new(bytes.Buffer)
    wc := io.MultiWriter(b1, b2)
    io.Copy(wc, request.Body)
    defer request.Body.Close()
    request1 = &http.Request{
        Method:        request.Method,
        URL:           request.URL,
        Proto:         request.Proto,
        ProtoMajor:    request.ProtoMajor,
        ProtoMinor:    request.ProtoMinor,
        Header:        request.Header,
        Body:          nopCloser{b1},
        Host:          request.Host,
        ContentLength: request.ContentLength,
        Close:         true,
    }
    request2 = &http.Request{
        Method:        request.Method,
        URL:           request.URL,
        Proto:         request.Proto,
        ProtoMajor:    request.ProtoMajor,
        ProtoMinor:    request.ProtoMinor,
        Header:        request.Header,
        Body:          nopCloser{b2},
        Host:          request.Host,
        ContentLength: request.ContentLength,
        Close:         true,
    }
    return
}

func (h httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    defer func() {
        if r := recover(); r != nil {
            if *debug {
                fmt.Println("Recovered ", r)
            }
        }
    }()
    mainReq, otherReq := DuplicateRequest(req) // dup the whole request
    b, _ := ioutil.ReadAll(otherReq.Body) // store the body once since it won't change
    for _, elem := range conf.Forwards {
        go func(el Target) {
            _, otherReq = DuplicateRequest(req)
            otherReq.Body = ioutil.NopCloser(bytes.NewBuffer(b)) // reset the body to original 
            MakeHTTPReq(string(el), otherReq, b, true) // ignore response and auto close for us
        }(elem)
    }
    resp, conn := MakeHTTPReq(string(conf.Proxy), mainReq, b, false) // forward the original req and get resp
    defer conn.Close()
    if resp != nil {
        fmt.Println(resp.Body)
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            if *debug {
                fmt.Printf("Failed to read resp from %s, %v\n", conf.Proxy, err)
            }
            return
        }
        defer resp.Body.Close() // close resp after reading body!!!
        if body == nil && *debug {
            fmt.Println("Empty Body Response")
        }
        for key, value := range resp.Header {
            w.Header()[key] = value // copy headers back
            fmt.Println(key, value)
        }
        w.WriteHeader(resp.StatusCode)
        w.Write(body) // write the response back
    }
}

func MakeHTTPReq(t string, req *http.Request, b []byte, autoClose bool) (resp *http.Response, httpConn *httputil.ClientConn){
    req.Body = ioutil.NopCloser(bytes.NewBuffer(b)) // reset the body to original 
    tcpConn, err := net.DialTimeout("tcp", t, time.Duration(time.Duration(10)*time.Second))
    if err != nil {
        if *debug {
            fmt.Printf("Can't make TCP connection to %s: %v\n", t, err)
        }
        return
    }
    httpConn = httputil.NewClientConn(tcpConn, nil)
    if autoClose {
        defer httpConn.Close()
    }
    err = httpConn.Write(req)
    if err != nil {
        if *debug {
            fmt.Printf("Failed to write http data to %s, %v\n", t, err)
        }
        return
    }
    resp, err = httpConn.Read(req)
    if err != nil && err != httputil.ErrPersistEOF {
        if *debug {
            fmt.Printf("Failed to read from %s: %v\n", t, err)
        }
        return
    }
    return
}

type httpHandler struct{}

// static definition of our config json
type Config struct{
    Listen Target        // must match top level keys in json file
    Proxy Target        // must match top level keys in json file
    Forwards []Target    // must match top level keys in json file
}

type Target string

var conf Config // for global access

func LoadJsonFromFile(file string, c *Config) (err error){
    if _, err = os.Stat(file); os.IsNotExist(err) {
        fmt.Printf("Config file '%s' does not exist!\n", file,)
        return err
    }
    fmt.Println("Using config file: ", file)
    content, err := ioutil.ReadFile(file)
    if err != nil {
        fmt.Print("Error reading config file: ", err)
        return err
    }
    err = json.Unmarshal(content, &c) // parse json into conf
        if err != nil {
        fmt.Print("Error in Json: ", err)
        return err
    }
    if *debug {
        fmt.Printf("Loaded JSON: %#v\n", c)
    }
    return err
}

func main(){
    flag.Parse()
    err := LoadJsonFromFile(*file, &conf)
    if err != nil {
        return
    }
    sigc := make(chan os.Signal, 1)
   	signal.Notify(sigc, syscall.SIGHUP)
    go func(){
        for {
            <- sigc // wait for channel to populate
            LoadJsonFromFile(*file, &conf)
            fmt.Println("Reloaded JSON!")
        }
    }()
    
    tcpListen, err := net.Listen("tcp", string(conf.Listen))
    fmt.Println("Listening on port: ", conf.Listen)
    http.Serve(tcpListen, httpHandler{})
}
