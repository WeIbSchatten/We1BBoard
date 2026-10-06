package controller

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RealityKeys generates an X25519 keypair + shortId for REALITY inbounds (3x-ui style).
func (a *API) RealityKeys(c *gin.Context) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		fail(c, 500, err)
		return
	}
	pub := priv.PublicKey()
	sid := make([]byte, 8)
	if _, err := rand.Read(sid); err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{
		"privateKey": base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		"publicKey":  base64.RawURLEncoding.EncodeToString(pub.Bytes()),
		"shortId":    hex.EncodeToString(sid),
	})
}

// RandomUUID returns a new UUID string for client forms.
func (a *API) RandomUUID(c *gin.Context) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	ok(c, gin.H{
		"uuid": hex.EncodeToString(b[0:4]) + "-" +
			hex.EncodeToString(b[4:6]) + "-" +
			hex.EncodeToString(b[6:8]) + "-" +
			hex.EncodeToString(b[8:10]) + "-" +
			hex.EncodeToString(b[10:16]),
	})
}
