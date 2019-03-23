package packages

import (
	"github.com/labstack/echo/v4"
	"github.com/poacpm/api.poac.pm/misc"
	"net/http"
)

func validateParam(name string, version string) error {
	err := misc.CheckPackageName(name)
	if err != nil {
		return err
	}
	err = misc.CheckPackageVersion(version)
	if err != nil {
		return err
	}
	return nil
}

func archiveImpl(c echo.Context, name string, version string) error {
	err := validateParam(name, version)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	url, err := misc.ArchiveUrl(c.Request(), name + "-" + version + ".tar.gz")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Redirect(http.StatusPermanentRedirect, url)
}

func Archive() echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("name")
		version := c.Param("version")
		return archiveImpl(c, name, version)
	}
}

func ArchiveOrg() echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("org") + "-" + c.Param("name")
		version := c.Param("version")
		return archiveImpl(c, name, version)
	}
}
