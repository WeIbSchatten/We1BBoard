package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"gopkg.in/yaml.v3"
)

type Server struct {
	Host string
}

func New() *Server { return &Server{} }

func (s *Server) Mount(r *gin.Engine, basePath string) {
	g := r.Group(basePath)
	g.GET("/:subId", s.handleRaw)
	g.GET("/:subId/json", s.handleJSON)
	g.GET("/:subId/clash", s.handleClash)
}

func (s *Server) clientsBySub(subID string) ([]model.Client, []model.Inbound, error) {
	var clients []model.Client
	if err := database.DB.Where("sub_id = ? AND enable = ?", subID, true).Find(&clients).Error; err != nil {
		return nil, nil, err
	}
	ids := map[uint]bool{}
	for _, c := range clients {
		ids[c.InboundID] = true
	}
	var inbounds []model.Inbound
	if len(ids) == 0 {
		return clients, inbounds, nil
	}
	idList := make([]uint, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	if err := database.DB.Where("id IN ? AND enable = ?", idList, true).Find(&inbounds).Error; err != nil {
		return nil, nil, err
	}
	return clients, inbounds, nil
}

func inboundMap(inbounds []model.Inbound) map[uint]model.Inbound {
	m := map[uint]model.Inbound{}
	for _, in := range inbounds {
		m[in.ID] = in
	}
	return m
}

func (s *Server) host() string {
	if s.Host != "" {
		return s.Host
	}
	h := database.GetSetting("subHost")
	if h != "" {
		return h
	}
	return "127.0.0.1"
}

func (s *Server) handleRaw(c *gin.Context) {
	clients, inbounds, err := s.clientsBySub(c.Param("subId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	imap := inboundMap(inbounds)
	host := s.host()
	var lines []string
	for _, cl := range clients {
		in, ok := imap[cl.InboundID]
		if !ok {
			continue
		}
		adap, err := protocol.Get(in.Protocol)
		if err != nil {
			continue
		}
		link, err := adap.ShareLink(&in, cl, host)
		if err != nil {
			continue
		}
		lines = append(lines, link)
	}
	body := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	c.String(http.StatusOK, body)
}

func (s *Server) handleJSON(c *gin.Context) {
	clients, inbounds, err := s.clientsBySub(c.Param("subId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	imap := inboundMap(inbounds)
	host := s.host()
	type item struct {
		Link     string `json:"link"`
		Protocol string `json:"protocol"`
		Email    string `json:"email"`
		Remark   string `json:"remark"`
	}
	out := []item{}
	for _, cl := range clients {
		in, ok := imap[cl.InboundID]
		if !ok {
			continue
		}
		adap, err := protocol.Get(in.Protocol)
		if err != nil {
			continue
		}
		link, err := adap.ShareLink(&in, cl, host)
		if err != nil {
			continue
		}
		out = append(out, item{Link: link, Protocol: string(in.Protocol), Email: cl.Email, Remark: in.Remark})
	}
	c.JSON(http.StatusOK, gin.H{"clients": out})
}

func (s *Server) handleClash(c *gin.Context) {
	clients, inbounds, err := s.clientsBySub(c.Param("subId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	imap := inboundMap(inbounds)
	host := s.host()
	proxies := []map[string]any{}
	names := []string{}
	for _, cl := range clients {
		in, ok := imap[cl.InboundID]
		if !ok {
			continue
		}
		name := cl.Email
		if name == "" {
			name = fmt.Sprintf("%s-%d", in.Protocol, cl.ID)
		}
		proxy := clashProxy(&in, cl, host, name)
		if proxy == nil {
			continue
		}
		proxies = append(proxies, proxy)
		names = append(names, name)
	}
	doc := map[string]any{
		"proxies": proxies,
		"proxy-groups": []map[string]any{
			{"name": "We1BBoard", "type": "select", "proxies": names},
		},
		"rules": []string{"MATCH,We1BBoard"},
	}
	b, _ := yaml.Marshal(doc)
	c.Data(http.StatusOK, "text/yaml; charset=utf-8", b)
}

func clashProxy(in *model.Inbound, cl model.Client, host, name string) map[string]any {
	stream := protocol.ParseStream(in.StreamSettings)
	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)
	switch in.Protocol {
	case model.ProtoVLESS:
		p := map[string]any{
			"name": name, "type": "vless", "server": host, "port": in.Port,
			"uuid": cl.UUID, "network": network, "udp": true,
		}
		if security == "reality" || security == "tls" {
			p["tls"] = true
		}
		if security == "reality" {
			p["reality-opts"] = map[string]any{}
			if rs, ok := stream["realitySettings"].(map[string]any); ok {
				if pbk, ok := rs["publicKey"].(string); ok {
					p["reality-opts"].(map[string]any)["public-key"] = pbk
				}
				if sid, ok := rs["shortIds"].([]any); ok && len(sid) > 0 {
					p["reality-opts"].(map[string]any)["short-id"] = fmt.Sprint(sid[0])
				}
			}
		}
		return p
	case model.ProtoTrojan:
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		return map[string]any{
			"name": name, "type": "trojan", "server": host, "port": in.Port,
			"password": pw, "udp": true,
		}
	case model.ProtoShadowsocks:
		settings := protocol.ParseSettings(in.Settings)
		method, _ := settings["method"].(string)
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		return map[string]any{
			"name": name, "type": "ss", "server": host, "port": in.Port,
			"cipher": method, "password": pw, "udp": true,
		}
	default:
		return nil
	}
}
