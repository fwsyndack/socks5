package main

import (
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net"
    "os"
    "strconv"
    "time"
)

//curl --socks5 127.0.0.1:1080 http://www.baidu.com

func main() {
    cmds := os.Args
    for i, cmd := range cmds {
        fmt.Printf("cmd[%d] = %s\n", i, cmd)
    }
    size := len(os.Args)
    fmt.Println("size:", size)

    startTCPServer()

}

func toInt(s string) int {
    n, err := strconv.Atoi(s)
    if err != nil {
        fmt.Println("Error:", err)
    }
    return n
}

func toFloat(s string) float64 {
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        fmt.Println("Error:", err)
    }
    return f
}

func toString(f float64) string {
    return strconv.FormatFloat(f, 'f', 1, 64)
}

func readBytes(filename string) []byte {
    f, err := os.Open(filename)
    if err != nil {
        fmt.Println("read file fail", err)
    }
    defer f.Close()
    bytes, err := ioutil.ReadAll(f)
    if err != nil {
        fmt.Println("read to bytes fail", err)
    }
    return bytes
}

//tcp server

func startTCPServer() {
    time.Sleep(time.Duration(1000) * time.Millisecond)

    server, err := net.Listen("tcp", ":1080")
    if err != nil {
        log.Fatal(err)
    }
    defer server.Close()
    fmt.Println("start socks5 server...")

    for {
        conn, err := server.Accept()
        if err != nil {
            fmt.Println("accept error:", err)
            log.Fatal(err)
        }

        fmt.Println("handleConn...")
        go handleConn(conn.(*net.TCPConn))
    }
}

func handleConn(conn *net.TCPConn) error {
    defer conn.Close()
    conn.SetNoDelay(true)

    //Authentication negotiation

    // Received:
    //               +----+----------+----------+
    //               |VER | NMETHODS | METHODS  |
    //               +----+----------+----------+
    //               | 1  |    1     | 1 to 255 |
    //               +----+----------+----------+
                   
    buf := make([]byte, 2)
    n, err := conn.Read(buf)
    if err != nil {
        log.Fatal(err)
    }

    if buf[0] != 0x05 {
        log.Println("Unsupported socks5.")
        log.Fatal(err)
    }
    fmt.Print("[RECEIVED]:   VER:", buf[0], " | NMETHODS:", buf[1])

    len := int(buf[1])
    buf = make([]byte, len)
    n, err = conn.Read(buf)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(" | METHODS:")
    fmt.Println(buf)

    // Reply:
    /*The server selects from one of the methods given in METHODS, and
    sends a METHOD selection message:

                         +----+--------+
                         |VER | METHOD |
                         +----+--------+
                         | 1  |   1    |
                         +----+--------+

    If the selected METHOD is X'FF', none of the methods listed by the
    client are acceptable, and the client MUST close the connection.

    The values currently defined for METHOD are:

          o  X'00' NO AUTHENTICATION REQUIRED
          o  X'01' GSSAPI
          o  X'02' USERNAME/PASSWORD
          o  X'03' to X'7F' IANA ASSIGNED
          o  X'80' to X'FE' RESERVED FOR PRIVATE METHODS
          o  X'FF' NO ACCEPTABLE METHODS

    The client and server then enter a method-specific sub-negotiation.
    */
                         
    resp := []byte{0x05, 0x00}
    n, err = write(resp, conn)
    if err != nil {
        log.Fatal(err)
    }

    ///////////////////////////////
    //Received Request from client
    /*
       The SOCKS request is formed as follows:

        +----+-----+-------+------+----------+----------+
        |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
        +----+-----+-------+------+----------+----------+
        | 1  |  1  | X'00' |  1   | Variable |    2     |
        +----+-----+-------+------+----------+----------+

     Where:

          o  VER    protocol version: X'05'
          o  CMD
             o  CONNECT X'01'
             o  BIND X'02'
             o  UDP ASSOCIATE X'03'
          o  RSV    RESERVED
          o  ATYP   address type of following address
             o  IP V4 address: X'01'
             o  DOMAINNAME: X'03'
             o  IP V6 address: X'04'
          o  DST.ADDR       desired destination address
          o  DST.PORT desired destination port in network octet
             order
    */
    
    buf = make([]byte, 4)
    n, err = conn.Read(buf)
    if err != nil {
        log.Fatal(err)
    }
    
    addrType := buf[3]
    if addrType == 0x01 {
        fmt.Println("address type is IPv4")    
    } else if addrType == 0x03 {
        fmt.Println("address type is Fully Qualified Domain Name")
    } else if addrType == 0x04 {
        fmt.Println("address type is IPv6")
    } else {
        fmt.Println("Unsupported address type")
        return fmt.Errorf("Unsupported address type: %d\n", addrType)
    }
    
    fmt.Print("[RECEIVED]:   VER:", buf[0], " | CMD:", buf[1], " | RSV:", buf[2], " | ATYP:", buf[3])
    if addrType == 0x01 { // IPv4
        buf = make([]byte, 4+2)
        n, err = conn.Read(buf)
        if err != nil {
            log.Fatal(err)
        }
        dstAddr := fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
        dstPort := (int(buf[4] & 0xFF) << 8) | int(buf[5] & 0xFF)
        fmt.Printf(" | DST_ADDR: %s | DST_PORT: %d\n", dstAddr, dstPort)

        // connect to DST_ADDR:DST_PORT
        remoteAddr := fmt.Sprintf("%s:%d", dstAddr, dstPort)
        fmt.Println("remoteAddr:", remoteAddr)

        remoteConn, err := net.Dial("tcp", remoteAddr)
        if err != nil {
            log.Fatal(err)
        }
        defer remoteConn.Close()

        local := remoteConn.LocalAddr().(*net.TCPAddr)
        bufLocalIP := local.IP.To4()
        localPort := local.Port;
        fmt.Printf("localAddr: %s:%d\n", local.IP, local.Port)
        
        // Reply:
        /*
        The SOCKS request information is sent by the client as soon as it has
        established a connection to the SOCKS server, and completed the
        authentication negotiations.  The server evaluates the request, and
        returns a reply formed as follows:

            +----+-----+-------+------+----------+----------+
            |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
            +----+-----+-------+------+----------+----------+
            | 1  |  1  | X'00' |  1   | Variable |    2     |
            +----+-----+-------+------+----------+----------+

        Where:

            o  VER    protocol version: X'05'
            o  REP    Reply field:
                o  X'00' succeeded
                o  X'01' general SOCKS server failure
                o  X'02' connection not allowed by ruleset
                o  X'03' Network unreachable
                o  X'04' Host unreachable
                o  X'05' Connection refused
                o  X'06' TTL expired
                o  X'07' Command not supported
                o  X'08' Address type not supported
                o  X'09' to X'FF' unassigned
            o  RSV    RESERVED
            o  ATYP   address type of following address
                o  IP V4 address: X'01'
                o  DOMAINNAME: X'03'
                o  IP V6 address: X'04'
            o  BND.ADDR       server bound address
            o  BND.PORT       server bound port in network octet order
        */
        resp := []byte{0x05, 0x00, 0x00, 0x01, bufLocalIP[0], bufLocalIP[1], bufLocalIP[2], bufLocalIP[3], byte((localPort >> 8) & 0xFF), byte(localPort & 0xFF)}
        _, err = write(resp, conn)
        if err != nil {
            log.Fatal(err)
        }
        
        //////////////////////////////////////////////
        
        errCh := make(chan error, 2)
        go ioCopy(remoteConn, conn, errCh)
        go ioCopy(conn, remoteConn, errCh)
        
        //Wait
        for i := 0; i < 2; i++ {
            e := <-errCh
            if e != nil {
                return e
            }
        }
        return nil
        
    } else if addrType == 0x03 { // Fully Qualified Domain Name
        /*
           the address field contains a fully-qualified domain name.  The first
        octet of the address field contains the number of octets of name that
        follow, there is no terminating NUL octet.
        */
        buf = []byte{0}
        n, err = conn.Read(buf)
        if err != nil {
            log.Println("err : ", err)
            return err
        }
        len := int(buf[0])
        
        bufFqdn := make([]byte, len)
        n, err = conn.Read(bufFqdn)
        if err != nil {
            log.Println("err : ", err)
            return err
        }
        fqdn := string(bufFqdn)
        
        fmt.Println("fqdn:", fqdn)
        //TODO
        
    } else if addrType == 0x04 { // IPv6
        buf = make([]byte, 16+2)
        n, err = conn.Read(buf)
        if err != nil {
            log.Println(n, err)
            return err
        }
        dstAddr := fmt.Sprintf("%x%x:%x%x:%x%x:%x%x:%x%x:%x%x:%x%x:%x%x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5], buf[6], buf[7], buf[8], buf[9], buf[10], buf[11], buf[12], buf[13], buf[14], buf[15])
        dstPort := (int(buf[16] & 0xFF) << 8) | int(buf[17] & 0xFF)
        fmt.Printf(" | DST_ADDR: %s | DST_PORT: %d\n", dstAddr, dstPort)
        //TODO
        // connect to DST_ADDR:DST_PORT
    }
    
    return nil
}

func write(data []byte, conn net.Conn) (n int, err error) {
    log.Println("Reply...", len(data))
    x := 0
    all := len(data)

    for all > 0 {
        n, err = conn.Write(data)
        if err != nil {
            n += x
            return
        }
        all -= n
        x += n
        data = data[n:]
    }

    return all, err
}

func ioCopy(dst io.Writer, src io.Reader, errCh chan error) {
    _, err := io.Copy(dst, src)
    
    if err != nil {
        log.Println("ioCopy:", err)
    }
    errCh <- err
}
