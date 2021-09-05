# Go2SheeP_ssh

Go2SheeP_ssh is a golang library which can help you to connect ssh server、establish tunnel proxy、use ssh as transport fast and easily.

_Go2SheeP means that the library can make you go to sleep earlier._

## usage example

1. connect to ssh server directly

   ```go
   import ssh "github.com/ciiiii/go2sheep_ssh"

   // Fill the &ssh.Info{} with your ssh sever info.
   func main() {
       client, err := ssh.New(&ssh.Info{}).Connect()
       if err != nil {
           panic(err)
       }
   }
   ```

1. connect to ssh server by bastion

   ```go
   import ssh "github.com/ciiiii/go2sheep_ssh"

   // Fill the &ssh.Info{} with your ssh sever info and bastion server info.
   // you can establish the ssh connection over bastion, and
   // the bastion connection also can be established over another bastion
   func main() {
       client, err := ssh.New(&ssh.Info{}).Bastion(&ssh.Info{}).Connect()
       if err != nil {
           panic(err)
       }
   }
   ```

1. use ssh tunneling proxy

   ```go
   import ssh "github.com/ciiiii/go2sheep_ssh"

   // The remote address(remoteHost:remotePort) can be the service that
   // your ssh server can access, then it will be access by the
   // local address(localHost:localPort).
   func main() {
       client, err := ssh.New(&ssh.Info{}).Connect()
       if err != nil {
           panic(err)
       }
       closeCh := make(chan os.Signal, 1)
       signal.Notify(closeCh, os.Interrupt, syscall.SIGTERM)
       if err := client.TunnelProxy(localHost, remoteHost, localPort, remotePort, closeCh); err != nil {
           panic(err)
       }
   }
   ```

1. use ssh client as http tranport

   ```go
   import ssh "github.com/ciiiii/go2sheep_ssh"

   // The ProxyHttpClient func will init a http client which can request
   // the url over the ssh connection.
   func main() {
       client, err := ssh.New(&ssh.Info{}).Connect()
       if err != nil {
           panic(err)
       }
       resp, err := client.ProxyHttpClient().Get(url)
       if err != nil {
           panic(err)
       }
   }
   ```

1. connect database over ssh proxy

   - mysql

     ```go
     import (
         "database/sql"
         ssh "github.com/ciiiii/go2sheep_ssh"
         "github.com/go-sql-driver/mysql"
     )

     func main() {
         client, err := ssh.New(&ssh.Info{}).Connect()
         if err != nil {
             panic(err)
         }
         // It also works for postgres.
         mysql.RegisterDial("mysql+tcp", (client.Dial)
         db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@mysql+tcp(%s)/%s", dbUser, dbPass, dbHost, dbName))
         if err != nil {
             panic(err)
         }
     }
     ```

   - redis

     ```go
     import (
         ssh "github.com/ciiiii/go2sheep_ssh"
         "module github.com/gomodule/redigo"
     )

     func main() {
        client, err := ssh.New(&ssh.Info{}).Connect()
        if err != nil {
            panic(err)
        }
        conn, err := client.Dial("tcp", redisAddr)
        if nil != err {
            panic(err)
        }
        redisConn := redis.NewConn(conn, -1, -1)
     }
     ```
