// Package controller exposes Gin HTTP handlers for the subconverter module.
//
// Response shape mirrors 3X-UI's existing controllers (entity.Msg) so the
// frontend has a single error-handling path across both surfaces.
package controller

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/web/entity"
	"github.com/mhsanaei/3x-ui/v3/web/locale"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/session"
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

// CheckAPIAuth mirrors 3X-UI's /panel/api authentication behaviour: accept
// panel sessions or Bearer API tokens, and hide API endpoints from
// unauthenticated users with 404 instead of redirecting.
func CheckAPIAuth(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		if (&service.ApiTokenService{}).Match(after) {
			if u, err := (&service.UserService{}).GetFirstUser(); err == nil {
				session.SetAPIAuthUser(c, u)
			}
			c.Set("api_authed", true)
			c.Next()
			return
		}
	}
	if !session.IsLogin(c) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Next()
}

func remoteIP(c *gin.Context) string {
	remoteIP, ok := extractTrustedIP(c.Request.RemoteAddr)
	if !ok {
		return ""
	}

	if isTrustedProxy(remoteIP) {
		if ip, ok := extractTrustedIP(c.GetHeader("X-Real-IP")); ok {
			return ip
		}
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			for part := range strings.SplitSeq(xff, ",") {
				if ip, ok := extractTrustedIP(part); ok {
					return ip
				}
			}
		}
	}

	return remoteIP
}

func isTrustedForwardedRequest(c *gin.Context) bool {
	remoteIP, ok := extractTrustedIP(c.Request.RemoteAddr)
	return ok && isTrustedProxy(remoteIP)
}

func isTrustedProxy(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	trusted := trustedProxyCIDRs()
	for value := range strings.SplitSeq(trusted, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if proxyIP, err := netip.ParseAddr(value); err == nil && proxyIP.Unmap() == addr.Unmap() {
			return true
		}
	}
	return false
}

func trustedProxyCIDRs() (trusted string) {
	trusted = "127.0.0.1/32,::1/128"
	defer func() {
		_ = recover()
	}()
	settingService := service.SettingService{}
	if value, err := settingService.GetTrustedProxyCIDRs(); err == nil && strings.TrimSpace(value) != "" {
		trusted = value
	}
	return trusted
}

func extractTrustedIP(value string) (string, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", false
	}

	if ip, ok := parseIPCandidate(candidate); ok {
		return ip.String(), true
	}

	if host, _, err := net.SplitHostPort(candidate); err == nil {
		if ip, ok := parseIPCandidate(host); ok {
			return ip.String(), true
		}
	}

	if strings.Count(candidate, ":") == 1 {
		if host, _, err := net.SplitHostPort(fmt.Sprintf("[%s]", candidate)); err == nil {
			if ip, ok := parseIPCandidate(host); ok {
				return ip.String(), true
			}
		}
	}

	return "", false
}

func parseIPCandidate(value string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}
