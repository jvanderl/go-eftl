package eftl

import (
	"github.com/gorilla/websocket"
	"github.com/op/go-logging"
	"encoding/json"
	"net/url"
//	"context"
	"errors"
	"strconv"
)
var log = logging.MustGetLogger("lib-go-eftl")

type eftlGeneric struct {
    element map[string]string
}
// {"op": 1, "client_type": "js", "client_version": "3.1.0   V7", "user":"user", "password":"password", "login_options": {"_qos": "true"}}


//type eftlLoginOptions struct {
//	_qos *bool
//}
type eftlLoginMessage struct {
	Operator int  `json:"op"`
	ClientType string `json:"client_type"`
	ClientVersion string `json:"client_version"`
	User string `json:"user"`
	Password string `json:"password"`
	LoginOptions map[string]string `json:"login_options"`
}

// {"op":2,"client_id":"68404BBF-8831-42CD-8744-43385AEF3590","id_token":"ZkGGc24IVFacA9jIGOBgb2bDc2g=","timeout":600,"heartbeat":240,"max_size":8192,"_qos":"true"}

type eftlLoginResponse struct {
	Operator int  `json:"op"`
	ClientID string `json:"client_id"`	
	IDToken string `json:"id_token"`
	Timeout int `json:"timeout"`
	Heartbeat int `json:"heartbeat"`
	MaxSize int `json:"max_size"`
	QoS string `json:"_qos"`
}

// {"op":3,"id":"68404BBF-8831-42CD-8744-43385AEF3590.s.1","matcher":"{\"_dest\":\"sample\"}"}

type eftlSubscription struct {
	Operator int  `json:"op"`
	ClientID string `json:"id"`	
	Matcher string `json:"matcher"`
}

// {"op":4,"id":"68404BBF-8831-42CD-8744-43385AEF3590.s.1"}

type eftlSubscriptionResponse struct {
	Operator int  `json:"op"`
	SubscriptionID string `json:"id"`	
}


// {"op":7,"to":"68404BBF-8831-42CD-8744-43385AEF3590.s.1","seq":1,"body":{"_dest":"sample","text":"This is a sample eFTL message","number":1}}

type eftlBody struct {
	Destination string `json:"_dest"`
	Text string `json:"text"`
	Number int `json:"number"`
}

type eftlMessage struct {
	Operator int  `json:"op"`
	To string `json:"to"`
	Sequence int `json:"seq"`
	Body eftlBody `json:"body"`
} 

// {"op":9,"seq":1}

type eftlSequenceMsg struct {
	Operator int  `json:"op"`
	Sequence int `json:"seq"`
}

func Login (conn websocket.Conn, user string, password string) (clientid string, idtoken string, err error){
	
	loginMessage := eftlLoginMessage{1, "js", "3.1.0   V7", user, password, map[string]string{"_qos": "true"}}

	loginb, err := json.Marshal(loginMessage)
	if err != nil {
		log.Debugf("Error while marshalling login message: [%s]", err)
		return "", "", err
	}
	
	log.Debug("Sending login message")

	err = conn.WriteMessage(websocket.TextMessage, loginb)
	if err != nil {
		log.Debugf("Error while sending login message to wsHost: [%s]", err)
		return "", "", err
	}

	msg, op := eftl.GetMessage (conn)
	switch op {
		case 2 : { // Login response


			// {"op":2,"client_id":"68404BBF-8831-42CD-8744-43385AEF3590","id_token":"ZkGGc24IVFacA9jIGOBgb2bDc2g=","timeout":600,"heartbeat":240,"max_size":8192,"_qos":"true"}
    		res := new(eftlLoginResponse)
		    if err := json.Unmarshal(msg, &res); err != nil {
				return "", "", err
			}
//							log.Debug("Login Response Received: [%s]", convert(p))
//				wsClientID = res.ClientID
//				wsIDToken = res.IDToken
//				log.Debugf("Login Succesful. client_id: [%s], id_token: [%s]", wsClientID, wsIDToken)
//							log.Debug("client_id:", wsClientID)
//							log.Debug("id_token:", wsIDToken)
			return res.ClientID, res.IDToken, nil


		}
		default: {
//			log.Debugf("Other message Received: [%s]", convert(msg))
		}

	}

	err = errors.New("No login response received")
	return "", "", err
}

func Connect(server string, channel string) (conn *websocket.Conn, err error) {

	wsURL := url.URL{Scheme: "ws", Host: server, Path: channel}
	log.Debugf("connecting to %s", wsURL.String())

	conn, _, err = websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
//		log.Debugf("Error while dialing to wsHost: ", err)
		return nil, err
	}
	return conn, nil
}

func Subscribe (conn websocket.Conn, clientid string, index int, destination string) (subscriptionid string, err error){

	subscriptionMessage := eftlSubscription{3, clientid + ".s." + strconv.Itoa(index) + "", "{\"_dest\":\"" + destination + "\"}"}

	subscrb, err := json.Marshal(subscriptionMessage)
	if err != nil {
		return "", err
	}
	
	err = conn.WriteMessage(websocket.TextMessage, subscrb)
	if err != nil {
		return "", err
	} 

	msg, op := eftl.GetMessage (conn)
	switch op {
		case 4 : { // subscription response
    		res := eftlSubscriptionResponse{}
		    if err := json.Unmarshal(msg, &res); err != nil {
				return "", err
			}
			return res.SubscriptionID, nil
		}
		default: {
			log.Debugf("Other message Received: [%s]", convert(msg))
		}
	}
	err = errors.New("No subscription response received")
	return "", err
}

func GetMessage (conn websocket.Conn) (message []byte, operator int){
	for {
		messageType, p, err := conn.ReadMessage()
		if err == nil {
			switch messageType {
		    	case websocket.TextMessage : {
		    		var dat map[string]interface{}
		    		if err := json.Unmarshal(p, &dat); err != nil {
     				   panic(err)
    				}
    				eftlOp := dat["op"].(float64)
    				if eftlOp != 0 { //Skip Heartbeat
    					return p, int(eftlOp)
    				}
		    	}
		    	case websocket.BinaryMessage : {
//		    		log.Debug("Received Binary message", p)
		    	}
		    	case websocket.CloseMessage : {
//		    		log.Debug("Received Close message", p)
		    		//return nil
		    	}
		    	case websocket.PingMessage : {
//		    		log.Debug("Received Ping message", p)
		    	}
		    	case websocket.PongMessage : {
//		    		log.Debug("Received Pong message", p)
		    	}
		    }
		} 
	}
}

func convert(b []byte) string {
	n := len(b)
	return string(b[:n])
}
