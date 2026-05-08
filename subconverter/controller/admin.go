package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v2/subconverter/service"
)

// AdminController serves /panel/api/subconverter/* admin endpoints.
//
// Route layout (mounted on a group that already has CheckAPIAuth applied):
//
//	GET  /list           list every subscription, newest-first
//	GET  /get/:id        single subscription with its inbound junction rows
//	POST /add            create (token is generated server-side)
//	POST /update/:id     replace mutable fields + the inbound list
//	POST /del/:id        delete subscription + junction rows + ip bindings
//
// Path layout matches 3X-UI's existing controllers (e.g. inbound.go).
type AdminController struct {
	subSvc *service.SubscriptionService
}

// NewPageController wires the HTML page routes for /panel/subconverter.
func NewPageController(g *gin.RouterGroup) {
	g.GET("", page)
	g.GET("/", page)
}

// NewAdminController wires routes onto the given group and returns the handle.
func NewAdminController(g *gin.RouterGroup) *AdminController {
	a := &AdminController{
		subSvc: service.NewSubscriptionService(),
	}
	a.initRouter(g)
	return a
}

func (a *AdminController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.delete)
}

// page renders the SPA shell (subconverter.html) which mounts a Vue 2 app.
// All data is fetched by the page itself via the JSON endpoints below.
func page(c *gin.Context) {
	renderHTML(c, "subconverter.html", "pages.subconverter.title")
}

func (a *AdminController) list(c *gin.Context) {
	subs, err := a.subSvc.List()
	if err != nil {
		jsonMsg(c, "list subscriptions failed", err)
		return
	}
	jsonObj(c, subs, nil)
}

func (a *AdminController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	sub, err := a.subSvc.Get(id)
	if err != nil {
		jsonMsg(c, "get subscription failed", err)
		return
	}
	jsonObj(c, sub, nil)
}

func (a *AdminController) add(c *gin.Context) {
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

func (a *AdminController) update(c *gin.Context) {
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

func (a *AdminController) delete(c *gin.Context) {
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
