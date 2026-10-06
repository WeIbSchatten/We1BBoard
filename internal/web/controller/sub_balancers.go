package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
)

var allowedSubBalancerStrategies = map[string]bool{
	"url-test":    true,
	"fallback":    true,
	"round-robin": true,
}

// ListSubBalancers GET /sub-balancers
func (a *API) ListSubBalancers(c *gin.Context) {
	var rows []model.SubBalancer
	if err := database.DB.Order("id asc").Find(&rows).Error; err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

// CreateSubBalancer POST /sub-balancers
func (a *API) CreateSubBalancer(c *gin.Context) {
	var b model.SubBalancer
	if err := c.ShouldBindJSON(&b); err != nil {
		fail(c, 400, err)
		return
	}
	if err := validateSubBalancer(&b); err != nil {
		fail(c, 400, err)
		return
	}
	if err := database.DB.Create(&b).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, b)
}

// UpdateSubBalancer PUT /sub-balancers/:id
func (a *API) UpdateSubBalancer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var b model.SubBalancer
	if err := c.ShouldBindJSON(&b); err != nil {
		fail(c, 400, err)
		return
	}
	b.ID = uint(id)
	if err := validateSubBalancer(&b); err != nil {
		fail(c, 400, err)
		return
	}
	if err := database.DB.Save(&b).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, b)
}

// DeleteSubBalancer DELETE /sub-balancers/:id
func (a *API) DeleteSubBalancer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := database.DB.Delete(&model.SubBalancer{}, id).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func validateSubBalancer(b *model.SubBalancer) error {
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" || len(b.Name) > 128 {
		return fmt.Errorf("invalid name")
	}
	b.Strategy = strings.ToLower(strings.TrimSpace(b.Strategy))
	if b.Strategy == "" {
		b.Strategy = "url-test"
	}
	if !allowedSubBalancerStrategies[b.Strategy] {
		return fmt.Errorf("strategy must be url-test, fallback, or round-robin")
	}
	if len(b.Selector) > 4096 {
		return fmt.Errorf("selector too long")
	}
	if len(b.Remark) > 255 {
		return fmt.Errorf("remark too long")
	}
	return nil
}
