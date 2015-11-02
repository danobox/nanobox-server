package main

import (
	"os"
	"net/http"
	"testing"
	"time"
	"io/ioutil"
	"runtime"
	"fmt"
	"encoding/json"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/jcelliott/lumber"

	"github.com/nanobox-io/nanobox-logtap/drain"
	"github.com/nanobox-io/nanobox-logtap/collector"


	"github.com/nanobox-io/golang-mist"
	"github.com/nanobox-io/nanobox-server/api"
	"github.com/nanobox-io/nanobox-server/config"
	"github.com/nanobox-io/nanobox-server/util/docker"
)

var apiClient = api.Init()

func TestMain(m *testing.M) {
	config.Log = lumber.NewConsoleLogger(lumber.DEBUG)

	curDir, err := os.Getwd()
	if err != nil {
		os.Exit(1)
	}
	config.MountFolder = curDir + "test/"
	config.DockerMount = curDir + "test/"
	config.App, _ = config.AppName()

	// // this is required testing docker things when not on linux
	// // im expecting some env var's to tell me how to connect
	// // see docker-machine
	// if runtime.GOOS != "linux" {
		
	// }
	docker.Client, _ = dc.NewClientFromEnv()
	
	config.Logtap.AddDrain("console", drain.AdaptLogger(config.Log))
	config.Logtap.AddDrain("mist", drain.AdaptPublisher(config.Mist))	
	// define logtap collectors/drains; we don't need to defer Close() anything here,
	// because these want to live as long as the server
	if _, err := collector.SyslogUDPStart("app", config.Ports["logtap"]+"1", config.Logtap); err != nil {
		panic(err)
	}

	//
	if _, err := collector.SyslogTCPStart("app", config.Ports["logtap"]+"1", config.Logtap); err != nil {
		panic(err)
	}

	// we will be adding a 0 to the end of the logtap port because we cant have 2 tcp listeneres
	// on the same port
	if _, err := collector.StartHttpCollector("deploy", config.Ports["logtap"]+"0", config.Logtap); err != nil {
		panic(err)
	}

	go func() {
		// start nanobox
		if err := apiClient.Start(config.Ports["api"]); err != nil {
			os.Exit(1)
		}
	}()
	<-time.After(time.Second)
	os.Exit(m.Run())
}

func TestPing(t *testing.T) {
	r, err := http.Get("http://localhost:1757/ping")
	if err != nil || r.StatusCode != 200 {
		t.Errorf("unable to ping")
	}
	bytes, _ := ioutil.ReadAll(r.Body)
	body := string(bytes)
	if body != "pong" {
		t.Errorf("expected pong but got %s", body)
	}
}

func TestDeploy(t *testing.T) {
	r, err := http.Post("http://localhost:1757/deploys?run=true", "json", nil)
	if err != nil || r.StatusCode != 200 {
		fmt.Println(r, err)
		t.Errorf("unable to deploy")
	}
	bytes, _ := ioutil.ReadAll(r.Body)
	deploy := map[string]string{}
	err = json.Unmarshal(bytes, &deploy)
	if err != nil {
		t.Errorf("unable to unmarshal body %s", bytes)
	}

	id := deploy["id"]

	mistClient := mist.NewLocalClient(config.Mist, 1)
	mistClient.Subscribe([]string{"job", "deploy"})

	message := <- mistClient.Messages()

	data := map[string]interface{}{}

	err = json.Unmarshal([]byte(message.Data), &data)
	if err != nil {
		t.Errorf("unable to unmarshal data %s\nerr: %s", message.Data, err.Error())
	}

	if data["document"].(map[string]interface{})["id"] != id {
		t.Errorf("the message is not for my deploy: %+v", data["document"])
	}
	if data["document"].(map[string]interface{})["status"] != "complete" {
		t.Errorf("I recieved a bad status: %+v", data["document"])
	}
}
