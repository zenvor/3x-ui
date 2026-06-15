package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	"github.com/mhsanaei/3x-ui/v3/subconverter/service"
)

// SubscriptionController serves /panel/api/subconverter/* subscription
// management endpoints.
//
// Route layout (mounted on a group that already has CheckAPIAuth applied):
//
//	GET  /list           list every subscription, newest-first
//	GET  /get/:id        subscription detail with IP bindings and access logs
//	GET  /logs           recent public feed access logs across subscriptions
//	GET  /logs/:id       recent public feed access logs
//	GET  /ips/:id        current IP bindings
//	POST /add            create (token is generated server-side)
//	POST /update/:id     replace mutable fields + the inbound list
//	POST /del/:id        delete subscription and related rows
//	POST /reset-token/:id replace token and clear IP bindings, stats, logs
//	POST /ips/:subscriptionId/del/:bindingId delete one IP binding
//	POST /ips/clear/:id  clear every IP binding under one subscription
//
// Path layout matches 3X-UI's existing controllers (e.g. inbound.go).
type SubscriptionController struct {
	subSvc      *service.SubscriptionService
	settingsSvc *service.SettingsService
	inboundOpts *service.InboundOptionService
	ipBindings  *service.IPBindingService
	accessLogs  *service.AccessLogService
}

// NewSubscriptionController wires routes onto the given group and returns the
// handle.
func NewSubscriptionController(g *gin.RouterGroup) *SubscriptionController {
	a := &SubscriptionController{
		subSvc:      service.NewSubscriptionService(),
		settingsSvc: service.NewSettingsService(),
		inboundOpts: service.NewInboundOptionService(),
		ipBindings:  service.NewIPBindingService(),
		accessLogs:  service.NewAccessLogService(),
	}
	a.initRouter(g)
	return a
}

func (a *SubscriptionController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)
	g.GET("/logs", a.allLogs)
	g.GET("/logs/:id", a.logs)
	g.GET("/ips/:id", a.ips)
	g.GET("/settings", a.settings)
	g.GET("/inbounds", a.inbounds)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.delete)
	g.POST("/ips/:subscriptionId/del/:bindingId", a.deleteIP)
	g.POST("/ips/clear/:id", a.clearIPs)
	g.POST("/reset-token/:id", a.resetToken)
	g.POST("/settings", a.updateSettings)
}

func (a *SubscriptionController) list(c *gin.Context) {
	subs, err := a.subSvc.List()
	if err != nil {
		jsonMsg(c, "list subscriptions failed", err)
		return
	}
	jsonObj(c, subs, nil)
}

func (a *SubscriptionController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	sub, err := a.subSvc.GetDetail(id)
	if err != nil {
		jsonMsg(c, "get subscription failed", err)
		return
	}
	jsonObj(c, sub, nil)
}

func (a *SubscriptionController) settings(c *gin.Context) {
	settings, err := a.settingsSvc.Get()
	if err != nil {
		jsonMsg(c, "get settings failed", err)
		return
	}
	jsonObj(c, settings, nil)
}

func (a *SubscriptionController) inbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	options, err := a.inboundOpts.List(user.Id)
	if err != nil {
		jsonMsg(c, "list inbounds failed", err)
		return
	}
	jsonObj(c, options, nil)
}

func (a *SubscriptionController) logs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	if _, err := a.subSvc.Get(id); err != nil {
		jsonMsg(c, "get subscription failed", err)
		return
	}
	logs, err := a.accessLogs.List(id)
	if err != nil {
		jsonMsg(c, "list access logs failed", err)
		return
	}
	jsonObj(c, logs, nil)
}

func (a *SubscriptionController) allLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	logs, err := a.accessLogs.ListRecent(limit)
	if err != nil {
		jsonMsg(c, "list access logs failed", err)
		return
	}
	jsonObj(c, logs, nil)
}

func (a *SubscriptionController) ips(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	if _, err := a.subSvc.Get(id); err != nil {
		jsonMsg(c, "get subscription failed", err)
		return
	}
	ips, err := a.ipBindings.List(id)
	if err != nil {
		jsonMsg(c, "list bound ips failed", err)
		return
	}
	jsonObj(c, ips, nil)
}

func (a *SubscriptionController) updateSettings(c *gin.Context) {
	var form service.SettingsInput
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "invalid request body", err)
		return
	}
	settings, err := a.settingsSvc.Update(form)
	if err != nil {
		jsonMsg(c, "update settings failed", err)
		return
	}
	jsonObj(c, settings, nil)
}

func (a *SubscriptionController) add(c *gin.Context) {
	var form service.SubscriptionFormInput
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "invalid request body", err)
		return
	}
	sub, err := a.subSvc.Create(form.ToInput())
	if err != nil {
		jsonMsg(c, "create subscription failed", err)
		return
	}
	jsonObj(c, sub, nil)
}

func (a *SubscriptionController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	var form service.SubscriptionFormInput
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "invalid request body", err)
		return
	}
	sub, err := a.subSvc.Update(id, form.ToInput())
	if err != nil {
		jsonMsg(c, "update subscription failed", err)
		return
	}
	jsonObj(c, sub, nil)
}

func (a *SubscriptionController) delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	if err := a.subSvc.Delete(id); err != nil {
		jsonMsg(c, "delete subscription failed", err)
		return
	}
	jsonMsg(c, i18nWeb(c, "pages.subconverter.deleted"), nil)
}

func (a *SubscriptionController) deleteIP(c *gin.Context) {
	subscriptionID, err := strconv.Atoi(c.Param("subscriptionId"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	bindingID, err := strconv.Atoi(c.Param("bindingId"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	if _, err := a.subSvc.Get(subscriptionID); err != nil {
		jsonMsg(c, "get subscription failed", err)
		return
	}
	if err := a.ipBindings.Delete(subscriptionID, bindingID); err != nil {
		jsonMsg(c, "delete bound ip failed", err)
		return
	}
	jsonMsg(c, i18nWeb(c, "pages.subconverter.deleted"), nil)
}

func (a *SubscriptionController) clearIPs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	if _, err := a.subSvc.Get(id); err != nil {
		jsonMsg(c, "get subscription failed", err)
		return
	}
	if err := a.ipBindings.Clear(id); err != nil {
		jsonMsg(c, "clear bound ips failed", err)
		return
	}
	jsonMsg(c, i18nWeb(c, "pages.subconverter.deleted"), nil)
}

func (a *SubscriptionController) resetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	sub, err := a.subSvc.ResetToken(id)
	if err != nil {
		jsonMsg(c, "reset token failed", err)
		return
	}
	jsonObj(c, sub, nil)
}
