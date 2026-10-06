package model

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Setting struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Key   string `gorm:"uniqueIndex;size:128;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

type Protocol string

const (
	ProtoVLESS       Protocol = "vless"
	ProtoVMess       Protocol = "vmess"
	ProtoTrojan      Protocol = "trojan"
	ProtoShadowsocks Protocol = "shadowsocks"
	ProtoWireGuard   Protocol = "wireguard"
	ProtoAmneziaWG   Protocol = "amneziawg"
	ProtoTUIC        Protocol = "tuic"
	ProtoHysteria2   Protocol = "hysteria2"
	ProtoMTProto     Protocol = "mtproto"
	ProtoHTTP        Protocol = "http"
	ProtoSOCKS       Protocol = "socks"
	ProtoTunnel      Protocol = "tunnel"
	ProtoTUN         Protocol = "tun"
)

type Inbound struct {
	ID             uint     `gorm:"primaryKey" json:"id"`
	Remark         string   `gorm:"size:255" json:"remark"`
	Enable         bool     `gorm:"default:true" json:"enable"`
	Listen         string   `gorm:"size:64;default:0.0.0.0" json:"listen"`
	Port           int      `gorm:"not null;index" json:"port"`
	Protocol       Protocol `gorm:"size:32;not null;index" json:"protocol"`
	Settings       string   `gorm:"type:text" json:"settings"`
	StreamSettings string   `gorm:"type:text" json:"streamSettings"`
	Sniffing       string   `gorm:"type:text" json:"sniffing"`
	Tag            string   `gorm:"uniqueIndex;size:128" json:"tag"`
	NodeID         *uint    `gorm:"index" json:"nodeId"`
	Up             int64    `json:"up"`
	Down           int64    `json:"down"`
	Total          int64    `json:"total"`
	ExpiryTime     int64    `json:"expiryTime"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Clients        []Client `gorm:"foreignKey:InboundID" json:"clients,omitempty"`
}

type Client struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InboundID  uint      `gorm:"index;not null" json:"inboundId"`
	Email      string    `gorm:"size:255;index" json:"email"`
	Enable     bool      `gorm:"default:true" json:"enable"`
	UUID       string    `gorm:"size:64" json:"uuid"`
	Password   string    `gorm:"size:255" json:"password"`
	Flow       string    `gorm:"size:64" json:"flow"`
	SubID      string    `gorm:"size:64;index" json:"subId"`
	TotalGB    int64     `json:"totalGB"`
	ExpiryTime int64     `json:"expiryTime"`
	Up         int64     `json:"up"`
	Down       int64     `json:"down"`
	TgID       int64     `json:"tgId"`
	Comment    string    `gorm:"size:512" json:"comment"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Outbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;size:128;not null" json:"tag"`
	Protocol  string    `gorm:"size:64;not null" json:"protocol"`
	Settings  string    `gorm:"type:text" json:"settings"`
	StreamSettings string `gorm:"type:text" json:"streamSettings"`
	Enable    bool      `gorm:"default:true" json:"enable"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Node struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	URL       string    `gorm:"size:512;not null" json:"url"`
	Token     string    `gorm:"size:255;not null" json:"token"`
	TLSMode   string    `gorm:"size:32;default:skip" json:"tlsMode"` // skip|verify|pin|mtls
	Region    string    `gorm:"size:32" json:"region"`               // ru|eu|other
	Enable    bool      `gorm:"default:true" json:"enable"`
	LastSeen  int64     `json:"lastSeen"`
	Online    bool      `json:"online"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Bridge struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Name           string `gorm:"size:128;not null" json:"name"`
	Enable         bool   `gorm:"default:true" json:"enable"`
	FromNodeID     *uint  `gorm:"index" json:"fromNodeId"` // nil = local (entry)
	ToNodeID       *uint  `gorm:"index" json:"toNodeId"`   // nil = local (exit)
	EntryInboundID *uint  `json:"entryInboundId"`
	ExitInboundTag string `gorm:"size:128" json:"exitInboundTag"`

	// Dialer = Xray outbound toward the exit node (any Xray outbound protocol).
	// Prefer Settings/StreamSettings JSON (3x-ui style); convenience fields fill defaults when empty.
	DialerProtocol       string `gorm:"size:32;default:vless" json:"dialerProtocol"`
	DialerAddress        string `gorm:"size:255" json:"dialerAddress"`
	DialerPort           int    `json:"dialerPort"`
	DialerUUID           string `gorm:"size:64" json:"dialerUUID"`
	DialerPassword       string `gorm:"size:255" json:"dialerPassword"`
	DialerEmail          string `gorm:"size:255" json:"dialerEmail"`
	DialerMethod         string `gorm:"size:64" json:"dialerMethod"` // shadowsocks
	DialerFlow           string `gorm:"size:64" json:"dialerFlow"`
	DialerSecurity       string `gorm:"size:32;default:none" json:"dialerSecurity"`
	DialerNetwork        string `gorm:"size:32;default:tcp" json:"dialerNetwork"`
	DialerSNI            string `gorm:"size:255" json:"dialerSNI"`
	DialerPublicKey      string `gorm:"size:128" json:"dialerPublicKey"`
	DialerShortID        string `gorm:"size:32" json:"dialerShortId"`
	DialerFingerprt      string `gorm:"size:64;default:chrome" json:"dialerFingerprint"`
	DialerPath           string `gorm:"size:255" json:"dialerPath"`
	DialerHost           string `gorm:"size:255" json:"dialerHost"`
	DialerServiceName    string `gorm:"size:255" json:"dialerServiceName"`
	DialerSettings       string `gorm:"type:text" json:"dialerSettings"`       // full Xray outbound settings JSON override
	DialerStreamSettings string `gorm:"type:text" json:"dialerStreamSettings"` // full streamSettings JSON override

	OutboundTag    string    `gorm:"size:128" json:"outboundTag"`
	RoutingInbound string    `gorm:"size:255" json:"routingInbound"` // comma-separated inbound tags
	Remark         string    `gorm:"size:512" json:"remark"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type TgProxyProfile struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:128;not null" json:"name"`
	Enable        bool      `gorm:"default:true" json:"enable"`
	Hostname      string    `gorm:"size:255;not null" json:"hostname"`
	Listen        string    `gorm:"size:64;default:127.0.0.1:8080" json:"listen"`
	MTProxyAddr   string    `gorm:"size:255;not null" json:"mtproxyAddr"`
	Secret        string    `gorm:"size:128;not null" json:"secret"`
	CarrierMode   string    `gorm:"size:64;default:websocket" json:"carrierMode"` // https|websocket|https_lanes|ws_lanes
	PublicSiteDir string    `gorm:"size:512" json:"publicSiteDir"`
	PublicMode    string    `gorm:"size:32;default:static" json:"publicMode"` // static|application
	AppUpstream   string    `gorm:"size:255" json:"appUpstream"`
	ConfigPath    string    `gorm:"size:512" json:"configPath"`
	Remark        string    `gorm:"size:512" json:"remark"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type RoutingRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Remark    string    `gorm:"size:255" json:"remark"`
	Enable    bool      `gorm:"default:true" json:"enable"`
	Priority  int       `gorm:"default:100" json:"priority"`
	InboundTag string   `gorm:"size:128" json:"inboundTag"`
	OutboundTag string  `gorm:"size:128;not null" json:"outboundTag"`
	Domain    string    `gorm:"type:text" json:"domain"`
	IP        string    `gorm:"type:text" json:"ip"`
	Port      string    `gorm:"size:64" json:"port"`
	Network   string    `gorm:"size:32" json:"network"`
	Protocol  string    `gorm:"size:64" json:"protocol"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
