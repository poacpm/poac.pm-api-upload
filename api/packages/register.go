package packages

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	"github.com/google/go-github/v27/github"
	"github.com/labstack/echo/v4"
	"github.com/poacpm/api.poac.pm/misc"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"unsafe"
)

type Config struct {
	// Load from poac.yml
	CppVersion float64 `json:"cpp_version"`
	Dependencies map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"dev_dependencies"`
	BuildDependencies map[string]string `json:"build_dependencies"`
	Build map[string]interface{} `json:"build"`
	Test map[string]interface{} `json:"test"`
	// Auto generate
	Owner string `json:"owner"`
	Repo string `json:"repo"`
	Version string `json:"version"`
	Description string `json:"description"`
	PackageType string `json:"package_type"`
	CommitSha string `json:"commit_sha"`
	Readme string `json:"readme"`
}

func structToJsonTagMap(data interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func registerPackage(r *http.Request, config *Config) error {
	configMap, err := structToJsonTagMap(config)
	if err != nil {
		return err
	}

	err = misc.CreateDoc(r, configMap)
	if err != nil {
		return err
	}
	return nil
}

func getReadme(c echo.Context, client *http.Client) string {
	url := "https://raw.githubusercontent.com/" + c.FormValue("owner") + "/" +
		   c.FormValue("repo") + "/" + c.FormValue("version") + "/README.md"
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	if resp.StatusCode == 404 {
		return ""
	}

	buffer, err := ioutil.ReadAll(resp.Body)
	return *(*string)(unsafe.Pointer(&buffer))
}

func getTestFromConfigYaml(configYaml map[string]interface{}) map[string]interface{} {
	if test, ok := configYaml["test"]; ok {
		return test.(map[string]interface{})
	}
	return nil
}

func getBuildFromConfigYaml(configYaml map[string]interface{}) map[string]interface{} {
	if build, ok := configYaml["build"]; ok {
		return build.(map[string]interface{})
	}
	return nil
}

func getDepsFromConfigYaml(configYaml map[string]interface{}, key string) map[string]string {
	if value, ok := configYaml[key]; ok {
		mapString := make(map[string]string)
		for key, value := range value.(map[string]interface{}) {
			strValue := fmt.Sprintf("%v", value)
			mapString[key] = strValue
		}
		return mapString
	}
	return nil
}

func validateCommitSha(c echo.Context, ctx context.Context, version string, client *github.Client) (string, error) {
	commitSha := c.FormValue("commit_sha")
	git, _, err := client.Git.GetRef(ctx, c.FormValue("owner"), c.FormValue("repo"), "refs/tags/" + version)
	if err != nil {
		return "", err
	}
	if commitSha != *git.Object.SHA {
		return "", errors.New("invalid commit-sha")
	}
	return commitSha, nil
}

func validatePackageType(c echo.Context, configYaml map[string]interface{}) (string, error) {
	packageType := c.FormValue("package_type")
	packageType2 := func(configYaml map[string]interface{}) string {
		if build, ok := configYaml["build"]; ok {
			if bin, ok := build.(map[string]interface{})["bin"]; ok {
				if bin.(bool) {
					return "application"
				}
			}
			return "build-required library"
		}
		return "header-only library"
	}(configYaml)

	if packageType != packageType2 {
		return "", nil
	}
	return packageType, nil
}

func validateCppVersion(c echo.Context, configYaml map[string]interface{}) (float64, error) {
	cppVersion := configYaml["cpp_version"].(float64)
	versions := []float64{98, 3, 11, 14, 17, 20}
	sort.Float64s(versions)
	i := sort.Search(len(versions),
		func(i int) bool { return versions[i] >= cppVersion })
	if !(i < len(versions) && versions[i] == cppVersion) {
		errStr := "Invalid cpp_version.\nPlease select one of 98, 03, 11, 14, 17, 20."
		return 0, errors.New(errStr)
	}

	if fmt.Sprint(cppVersion) != c.FormValue("cpp_version") {
		errStr := "Invalid cpp_version.\npoac.yml and form do not match cpp_version."
		return 0, errors.New(errStr)
	}
	return cppVersion, nil
}

func validateDescription(c echo.Context, ctx context.Context, client *github.Client) (string, error) {
	description := c.FormValue("description")
	repo, _, err := client.Repositories.Get(ctx, c.FormValue("owner"), c.FormValue("repo"))
	if err != nil {
		return "", err
	}
	if description != *repo.Description  {
		return "", errors.New("invalid description")
	}

	if description == "null" {
		return "", nil
	}
	return description, nil
}

func verifyExists(c echo.Context, version string) error {
	isExists, err := handleExists(c, c.FormValue("owner"), c.FormValue("repo"), version)
	if err == nil {
		return err
	}

	if isExists {
		return errors.New("Already " + c.FormValue("owner") + "/" +  c.FormValue("repo") + ": " + version + "is exists")
	} else {
		return nil
	}
}

func validateVersion(c echo.Context) (string, error) {
	version := c.FormValue("version")
	// semver error
	_, err := semver.Make(version)
	if err != nil {
		errStr := "Invalid version.\nPlease adapt to semver.\nSee https://semver.org for details."
		return "", errors.New(errStr)
	}
	return version, nil
}

func validateRepo(c echo.Context) (string, error) {
	repo := c.FormValue("repo")
	if !regexp.MustCompile("^([a-z|\\d|\\-|_]*)$").Match([]byte(repo)) {
		errStr := "Invalid name.\nIt is prohibited to use a character string that does not match ^([a-z|\\d|\\-|_]*)$ in the project name."
		return "", errors.New(errStr)
	}

	// Ban keywords
	// https://en.cppreference.com/w/cpp/keyword
	blacklist := []string{
		"alignas", "alignof", "and", "and_eq", "asm", "atomic_cancel", "atomic_commit", "atomic_noexcept",
		"auto", "bitand", "bitor", "bool", "break", "case", "catch", "char", "char8_t", "char16_t", "char32_t",
		"class", "compl", "concept", "const", "consteval", "constexpr", "const_cast", "continue", "co_await",
		"co_return", "co_yield", "decltype", "default", "delete", "do", "double", "dynamic_cast", "else", "enum",
		"explicit", "export", "extern", "false", "float", "for", "friend", "goto", "if", "inline", "int", "long",
		"mutable", "namespace", "new", "noexcept", "not", "not_eq", "nullptr", "operator", "or", "or_eq", "private",
		"protected", "public", "reflexpr", "register", "reinterpret_cast", "requires", "return", "short", "signed",
		"sizeof", "static", "static_assert", "static_cast", "struct", "switch", "synchronized", "template", "this",
		"thread_local", "throw", "true", "try", "typedef", "typeid", "typename", "union", "unsigned", "using",
		"virtual", "void", "volatile", "wchar_t", "while", "xor", "xor_eq",
	}
	for _, v := range blacklist {
		if repo == v {
			errStr := v + "is a keyword, so it cannot be used as a package name."
			return "", errors.New(errStr)
		}
	}
	return repo, nil
}

func openConfigFile(c echo.Context) (map[string]interface{}, error) {
	configFile, err := c.FormFile("poac.yml")
	if err != nil {
		return nil, err
	}

	configFile2, err := configFile.Open()
	defer configFile2.Close()
	if err != nil {
		return nil, err
	}

	configBytes, err := ioutil.ReadAll(configFile2)
	if err != nil {
		return nil, err
	}

	configYaml := make(map[string]interface{})
	err = yaml.Unmarshal(configBytes, &configYaml)
	if err != nil {
		return nil, err
	}
	return configYaml, nil
}

func validateConfig(c echo.Context, ctx context.Context, client *github.Client) (Config, error) {
	config := Config{}
	configYaml, err := openConfigFile(c)
	if err != nil {
		return Config{}, err
	}

	config.Owner = c.FormValue("owner")
	config.Repo, err = validateRepo(c)
	if err != nil {
		return Config{}, err
	}

	config.Version, err = validateVersion(c)
	if err != nil {
		return Config{}, err
	}

	err = verifyExists(c, config.Version)
	if err != nil {
		return Config{}, err
	}

	config.Description, err = validateDescription(c, ctx, client)
	if err != nil {
		return Config{}, err
	}

	config.CppVersion, err = validateCppVersion(c, configYaml)
	if err != nil {
		return Config{}, err
	}

	config.PackageType, err = validatePackageType(c, configYaml)
	if err != nil {
		return Config{}, err
	}

	config.CommitSha, err = validateCommitSha(c, ctx, config.Version, client)
	if err != nil {
		return Config{}, err
	}

	config.Dependencies = getDepsFromConfigYaml(configYaml, "dependencies")
	config.DevDependencies = getDepsFromConfigYaml(configYaml, "dev_dependencies")
	config.BuildDependencies = getDepsFromConfigYaml(configYaml, "build_dependencies")
	config.Build = getBuildFromConfigYaml(configYaml)
	config.Test = getTestFromConfigYaml(configYaml)
	config.Readme = getReadme(c, urlfetch.Client(ctx))

	return config, nil
}

func Register() echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := appengine.NewContext(c.Request())
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("POAC_GITHUB_API_TOKEN")},
		)
		tc := oauth2.NewClient(ctx, ts)
		client := github.NewClient(tc)

		config, err := validateConfig(c, ctx, client)
		if err != nil {
			return err
		}

		err = registerPackage(c.Request(), &config)
		if err != nil {
			return err
		}
		return c.String(http.StatusOK, "ok")
	}
}
