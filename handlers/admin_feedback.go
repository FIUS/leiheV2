package handlers

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func redirectAdminNotice(c *gin.Context, fallbackPath, kind, message string) {
	target := fallbackPath
	var targetQuery string
	var targetFragment string

	if referrer := c.GetHeader("Referer"); referrer != "" {
		if parsed, err := url.Parse(referrer); err == nil {
			reqHost := c.Request.Host
			sameHost := parsed.Host == "" || strings.EqualFold(parsed.Host, reqHost)
			if !sameHost && reqHost != "" && parsed.Host != "" {
				parsedHostname := parsed.Hostname()
				reqHostname := reqHost
				if h, _, err := net.SplitHostPort(reqHost); err == nil {
					reqHostname = h
				}
				if strings.EqualFold(parsedHostname, reqHostname) {
					sameHost = true
				}
			}
			if sameHost && (strings.HasPrefix(parsed.Path, "/admin/") || parsed.Path == "/admin" || parsed.Path == "/setup-bilder") {
				target = parsed.Path
				targetQuery = parsed.RawQuery
				targetFragment = parsed.Fragment
			}
		}
	}

	values, _ := url.ParseQuery(targetQuery)
	values.Set("notice", message)
	values.Set("notice_type", kind)

	finalURL := target
	if encoded := values.Encode(); encoded != "" {
		finalURL += "?" + encoded
	}
	if targetFragment != "" {
		finalURL += "#" + targetFragment
	}
	c.Redirect(http.StatusSeeOther, finalURL)
}
