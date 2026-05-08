// Package controller exposes Gin HTTP handlers for the subconverter module.
//
// Response shape mirrors 3X-UI's existing controllers (entity.Msg) so the
// frontend has a single error-handling path across both surfaces.
package controller

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
	"github.com/mhsanaei/3x-ui/v2/web/locale"
	"github.com/mhsanaei/3x-ui/v2/web/session"
)

// jsonMsg sends an entity.Msg with no payload. err == nil means success.
func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

// jsonObj sends an entity.Msg whose Obj is the given payload. err == nil means
// success.
func jsonObj(c *gin.Context, obj any, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj any, err error) {
	m := entity.Msg{Obj: obj}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		m.Msg = msg
		if err.Error() != "" {
			if msg != "" {
				m.Msg = msg + " (" + err.Error() + ")"
			} else {
				m.Msg = err.Error()
			}
		}
	}
	c.JSON(http.StatusOK, m)
}

// renderHTML serves a Go-template HTML page using the same context shape
// 3X-UI's web/controller.html() builds (title, host, request_uri, base_path,
// cur_ver). Replicated here so the subconverter package does not import the
// heavyweight controller package.
func renderHTML(c *gin.Context, name, titleKey string) {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.GetHeader("X-Real-IP")
	}
	if host == "" {
		if h, _, err := net.SplitHostPort(c.Request.Host); err == nil {
			host = h
		} else {
			host = c.Request.Host
		}
	}
	c.HTML(http.StatusOK, name, gin.H{
		"title":       titleKey,
		"host":        host,
		"request_uri": c.Request.RequestURI,
		"base_path":   c.GetString("base_path"),
		"cur_ver":     config.GetVersion(),
	})
}

func i18nWeb(c *gin.Context, key string, params ...string) string {
	anyfunc, ok := c.Get("I18n")
	if !ok {
		return key
	}
	i18nFunc, ok := anyfunc.(func(locale.I18nType, string, ...string) string)
	if !ok {
		return key
	}
	return i18nFunc(locale.Web, key, params...)
}

// CheckLogin is a Gin middleware that requires a valid 3X-UI session.
// Mirrors web/controller.BaseController.checkLogin so the subconverter package
// does not need to depend on the heavy controller package.
func CheckLogin(c *gin.Context) {
	if session.IsLogin(c) {
		c.Next()
		return
	}
	if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
		c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		c.Abort()
		return
	}
	c.JSON(http.StatusUnauthorized, entity.Msg{Success: false, Msg: i18nWeb(c, "pages.login.loginAgain")})
	c.Abort()
}

// CheckAPIAuth mirrors 3X-UI's /panel/api authentication behaviour: hide API
// endpoints from unauthenticated users with 404 instead of redirecting.
func CheckAPIAuth(c *gin.Context) {
	if !session.IsLogin(c) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Next()
}
