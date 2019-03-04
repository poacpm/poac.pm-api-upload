package route

import (
	"github.com/labstack/echo/v4"
	"github.com/poacpm/api.poac.pm/api"
	"github.com/poacpm/api.poac.pm/api/packages"
	"github.com/poacpm/api.poac.pm/api/tokens"
	_ "github.com/poacpm/api.poac.pm/docs"
	"github.com/poacpm/api.poac.pm/middleware"
	"github.com/swaggo/echo-swagger/v2"
)

func Init() *echo.Echo {
	e := echo.New()
	middleware.Set(e)

	// Routes
	packagesRoute := e.Group("/packages")
	{
		packagesRoute.GET("/:name/:version/deps", packages.Deps())
		packagesRoute.GET("/:org/:name/:version/deps", packages.DepsOrg())
		packagesRoute.GET("/:name/versions", packages.Versions())
		packagesRoute.GET("/:org/:name/versions", packages.VersionsOrg())
		packagesRoute.GET("/:name/:version/exists", packages.Exists())
		packagesRoute.GET("/:org/:name/:version/exists", packages.ExistsOrg())
		packagesRoute.POST("/upload", packages.Upload())
	}
	e.POST("/tokens/validate", tokens.Validate())
	e.GET("/statuspage", api.Statuspage())
	e.GET("/swagger/*", echoSwagger.WrapHandler)
	return e
}