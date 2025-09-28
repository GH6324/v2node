package panel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"encoding/json"
)

// Security type
const (
	None    = 0
	Tls     = 1
	Reality = 2
)

type NodeInfo struct {
	Id           int
	Type         string
	Security     int
	PushInterval time.Duration
	PullInterval time.Duration
	DNS          DNS
	Rules        Rules

	Common *CommonNode
}

type CommonNode struct {
	Protocol   string      `json:"protocol"`
	ListenIP   string      `json:"listen_ip"`
	ServerPort int         `json:"server_port"`
	Routes     []Route     `json:"routes"`
	BaseConfig *BaseConfig `json:"base_config"`
	//vless vmess trojan
	Tls                int         `json:"tls"`
	TlsSettings        TlsSettings `json:"tls_settings"`
	CertInfo           *CertInfo
	Network            string          `json:"network"`
	NetworkSettings    json.RawMessage `json:"networkSettings"`
	Encryption         string          `json:"encryption"`
	EncryptionSettings EncSettings     `json:"encryption_settings"`
	ServerName         string          `json:"server_name"`
	Flow               string          `json:"flow"`
	//shadowsocks
	Cipher    string `json:"cipher"`
	ServerKey string `json:"server_key"`
	//tuic
	CongestionControl string `json:"congestion_control"`
	ZeroRTTHandshake  bool   `json:"zero_rtt_handshake"`
	//anytls
	PaddingScheme []string `json:"padding_scheme,omitempty"`
	//hysteria hysteria2
	UpMbps                  int    `json:"up_mbps"`
	DownMbps                int    `json:"down_mbps"`
	Obfs                    string `json:"obfs"`
	ObfsPassword            string `json:"obfs-password"`
	Ignore_Client_Bandwidth bool   `json:"ignore_client_bandwidth"`
}

type Route struct {
	Id          int         `json:"id"`
	Match       interface{} `json:"match"`
	Action      string      `json:"action"`
	ActionValue string      `json:"action_value"`
}
type BaseConfig struct {
	PushInterval           any `json:"push_interval"`
	PullInterval           any `json:"pull_interval"`
	DeviceOnlineMinTraffic int `json:"device_online_min_traffic"`
}

type TlsSettings struct {
	ServerName       string `json:"server_name"`
	Dest             string `json:"dest"`
	ServerPort       string `json:"server_port"`
	ShortId          string `json:"short_id"`
	PrivateKey       string `json:"private_key"`
	Mldsa65Seed      string `json:"mldsa65Seed"`
	Xver             uint64 `json:"xver,string"`
	CertMode         string `json:"cert_mode"`
	Provider         string `json:"provider"`
	DNSEnv           string `json:"dns_env"`
	RejectUnknownSni bool   `json:"reject_unknown_sni"`
}

type CertInfo struct {
	CertMode         string
	CertFile         string
	KeyFile          string
	Email            string
	CertDomain       string
	DNSEnv           map[string]string
	Provider         string
	RejectUnknownSni bool
}

type EncSettings struct {
	Mode          string `json:"mode"`
	Ticket        string `json:"ticket"`
	ServerPadding string `json:"server_padding"`
	PrivateKey    string `json:"private_key"`
}

type DNS struct {
	DNSMap  map[string]map[string]interface{}
	DNSJson []byte
}

type Rules struct {
	Regexp   []string
	Protocol []string
}

func (c *Client) GetNodeInfo() (node *NodeInfo, err error) {
	const path = "/api/v2/server/config"
	r, err := c.client.
		R().
		SetHeader("If-None-Match", c.nodeEtag).
		ForceContentType("application/json").
		Get(path)

	if r.StatusCode() == 304 {
		return nil, nil
	}
	hash := sha256.Sum256(r.Body())
	newBodyHash := hex.EncodeToString(hash[:])
	if c.responseBodyHash == newBodyHash {
		return nil, nil
	}
	c.responseBodyHash = newBodyHash
	c.nodeEtag = r.Header().Get("ETag")
	if err = c.checkResponse(r, path, err); err != nil {
		return nil, err
	}

	if r != nil {
		defer func() {
			if r.RawBody() != nil {
				r.RawBody().Close()
			}
		}()
	} else {
		return nil, fmt.Errorf("received nil response")
	}
	node = &NodeInfo{
		Id: c.NodeId,
		DNS: DNS{
			DNSMap:  make(map[string]map[string]interface{}),
			DNSJson: []byte(""),
		},
	}
	// parse protocol params
	cm := &CommonNode{}
	err = json.Unmarshal(r.Body(), cm)
	if err != nil {
		return nil, fmt.Errorf("decode node params error: %s", err)
	}
	switch cm.Protocol {
	case "vmess", "trojan", "hysteria2", "tuic", "anytls", "vless":
		node.Type = cm.Protocol
		node.Security = cm.Tls
	case "shadowsocks":
		node.Type = cm.Protocol
		node.Security = 0
	default:
		return nil, fmt.Errorf("unsupport protocol: %s", cm.Protocol)
	}
	
	cm.CertInfo = &CertInfo{
		CertMode:         cm.TlsSettings.CertMode,
		Email:            "node@v2board.com",
		CertDomain:       cm.TlsSettings.ServerName,
		DNSEnv:           make(map[string]string),
		Provider:         cm.TlsSettings.Provider,
		RejectUnknownSni: cm.TlsSettings.RejectUnknownSni,
	}
	if cm.CertInfo.CertMode == "dns" && cm.TlsSettings.DNSEnv != "" {
		envs := strings.Split(cm.TlsSettings.DNSEnv, ",")
		for _, env := range envs {
			kv := strings.SplitN(env, "=", 2)
			if len(kv) == 2 {
				cm.CertInfo.DNSEnv[kv[0]] = kv[1]
			}
		}
	}
	// parse rules and dns
	for i := range cm.Routes {
		var matchs []string
		if _, ok := cm.Routes[i].Match.(string); ok {
			matchs = strings.Split(cm.Routes[i].Match.(string), ",")
		} else if _, ok = cm.Routes[i].Match.([]string); ok {
			matchs = cm.Routes[i].Match.([]string)
		} else {
			temp := cm.Routes[i].Match.([]interface{})
			matchs = make([]string, len(temp))
			for i := range temp {
				matchs[i] = temp[i].(string)
			}
		}
		switch cm.Routes[i].Action {
		case "block":
			for _, v := range matchs {
				if strings.HasPrefix(v, "protocol:") {
					// protocol
					node.Rules.Protocol = append(node.Rules.Protocol, strings.TrimPrefix(v, "protocol:"))
				} else {
					// domain
					node.Rules.Regexp = append(node.Rules.Regexp, strings.TrimPrefix(v, "regexp:"))
				}
			}
		case "dns":
			var domains []string
			domains = append(domains, matchs...)
			if matchs[0] != "main" {
				node.DNS.DNSMap[strconv.Itoa(i)] = map[string]interface{}{
					"address": cm.Routes[i].ActionValue,
					"domains": domains,
				}
			} else {
				dns := []byte(strings.Join(matchs[1:], ""))
				node.DNS.DNSJson = dns
			}
		}
	}

	// set interval
	node.PushInterval = intervalToTime(cm.BaseConfig.PushInterval)
	node.PullInterval = intervalToTime(cm.BaseConfig.PullInterval)

	node.Common = cm
	// clear
	cm.Routes = nil
	cm.BaseConfig = nil

	return node, nil
}

func intervalToTime(i interface{}) time.Duration {
	switch reflect.TypeOf(i).Kind() {
	case reflect.Int:
		return time.Duration(i.(int)) * time.Second
	case reflect.String:
		i, _ := strconv.Atoi(i.(string))
		return time.Duration(i) * time.Second
	case reflect.Float64:
		return time.Duration(i.(float64)) * time.Second
	default:
		return time.Duration(reflect.ValueOf(i).Int()) * time.Second
	}
}
