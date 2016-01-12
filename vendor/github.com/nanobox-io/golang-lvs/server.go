package lvs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type (
	Server struct {
		Host                string `json:"host"`
		Port                int    `json:"port"`
		Forwarder           string `json:"forwarder"`
		Weight              int    `json:"weight"`
		UpperThreshold      int    `json:upper_threshold`
		LowerThreshold      int    `json:lower_threshold`
		InactiveConnections int    `json:"innactive_connections"`
		ActiveConnections   int    `json:"active_connections"`
	}
)

var (
	ServerForwarderFlag = map[string]string{
		"g": "-g",
		"i": "-i",
		"m": "-m",
		"":  "-g", // default
	}
)

func (s *Server) FromJson(bytes []byte) error {
	return json.Unmarshal(bytes, s)
}

func (s Server) ToJson() ([]byte, error) {
	return json.Marshal(s)
}

func (s Server) getHostPort() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s Server) String() string {
	return fmt.Sprintf("%s %s -y %s -x %s -w %s",
		s.getHostPort(), ServerForwarderFlag[s.Forwarder],
		s.LowerThreshold, s.UpperThreshold, s.Weight)
}

func parseServer(serverString string) Server {
	server := Server{
		Forwarder: "g",
		Weight:    1,
	}
	var err error
	exploded := strings.Split(serverString, " ")
	for i := range exploded {
		switch exploded[i] {
		case "-r", "--real-server":
			server.Host, server.Port = parseHostPort(exploded[i+1])
		case "-g", "--gatewaying":
			server.Forwarder = "g"
		case "-i", "--ipip":
			server.Forwarder = "i"
		case "-m", "--masquerading":
			server.Forwarder = "m"
		case "-w", "--weight":
			server.Weight, err = strconv.Atoi(exploded[i+1])
			if err != nil {
				server.Weight = 1
			}
		case "-x", "--u-threshold":
			server.UpperThreshold, err = strconv.Atoi(exploded[i+1])
			if err != nil {
				server.UpperThreshold = 0
			}
		case "-y", "--l-threshold":
			server.LowerThreshold, err = strconv.Atoi(exploded[i+1])
			if err != nil {
				server.LowerThreshold = 0
			}
		}
	}
	return server
}
