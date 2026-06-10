// Package subconverter integrates a Mihomo subscription generator into the
// 3X-UI panel.
//
// Layout:
//   - Admin SPA:        {basePath}panel/subconverter     (login required)
//   - Admin API:        {basePath}panel/api/subconverter/* (login required)
//   - Public Mihomo sub: /feed/:token                    (Mihomo YAML)
//   - Public Mihomo sub: /feed/:token/nodes              (Mihomo proxy-provider)
//
// Persistence is in a separate SQLite file (data/subconverter.db) so that
// upstream merges and 3X-UI's getDb/importDB do not interact with our data.
package subconverter

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/subconverter/controller"
	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/web/middleware"
)

// RegisterRoutes initializes the subconverter database and mounts every route
// the module exposes.
//
//   - engine is the root Gin engine; public /feed/* endpoints (which must not
//     sit under the panel basePath) attach in later stages.
//   - panelGroup is the basePath group used by 3X-UI's panel; admin endpoints
//     and the SPA attach here.
//
// This is the single entry point that web/web.go calls. Keeping it as one
// function minimizes the upstream touch surface to one line in web/web.go.
func RegisterRoutes(engine *gin.Engine, panelGroup *gin.RouterGroup) error {
	if err := database.InitDB(); err != nil {
		return err
	}

	api := panelGroup.Group("/panel/api/subconverter")
	api.Use(controller.CheckAPIAuth)
	api.Use(middleware.CSRFMiddleware())
	controller.NewSubscriptionController(api)

	// Public Mihomo endpoints. These intentionally sit on the root engine
	// (not under basePath) and have no auth: clients request with just the token.
	controller.NewPublicController(engine)

	return nil
}
