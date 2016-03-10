package configstore

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestJSONConfigWriterProvider(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.NoError(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	tmpDir, err := ioutil.TempDir("", "fissile-config-json-tests")
	assert.NoError(err)
	defer os.RemoveAll(tmpDir)
	outDir := filepath.Join(tmpDir, "store")

	builder := NewConfigStoreBuilder(JSONProvider, opinionsFile, opinionsFileDark, outDir)

	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	assert.NoError(err)

	roleManifestPath := filepath.Join(workDir, "../test-assets/role-manifests/tor-good.yml")
	rolesManifest, err := model.LoadRoleManifest(roleManifestPath, []*model.Release{release})
	assert.NoError(err)

	err = builder.WriteBaseConfig(rolesManifest)
	assert.NoError(err)

	jsonPath := filepath.Join(outDir, "myrole", "new_hostname.json")

	buf, err := ioutil.ReadFile(jsonPath)
	if !assert.NoError(err, "Failed to read output %s\n", jsonPath) {
		return
	}

	var result map[string]interface{}
	err = json.Unmarshal(buf, &result)
	if !assert.NoError(err, "Error unmarshalling output") {
		return
	}

	assert.Equal("myrole", result["job"].(map[string]interface{})["name"])

	templates := result["job"].(map[string]interface{})["templates"]
	assert.Contains(templates, map[string]interface{}{"name": "tor"})
	assert.Contains(templates, map[string]interface{}{"name": "new_hostname"})
	assert.Len(templates, 2)

	var expected map[string]interface{}
	err = json.Unmarshal([]byte(`{
		   "tor": {
            "client_keys": null,
            "hashed_control_password": null,
            "hostname": null,
            "private_key": null
        }
	}`), &expected)
	assert.NoError(err, "Failed to unmarshal expected data")

	assert.Equal(expected, result["properties"], "Unexpected properties")
}

func TestInitializeConfigJSON(t *testing.T) {
	assert := assert.New(t)

	config, err := initializeConfigJSON()
	assert.NoError(err)

	jobConfig, ok := config["job"].(map[string]interface{})
	assert.True(ok, "Job config should be a map with string keys")
	assert.NotNil(jobConfig["templates"])
	_, ok = jobConfig["templates"].([]interface{})
	assert.True(ok, "Job templates should be an array")

	_, ok = config["parameters"].(map[string]interface{})
	assert.True(ok, "Parameters should be a map")

	_, ok = config["properties"].(map[string]interface{})
	assert.True(ok, "Properties should be a map")

	networks, ok := config["networks"].(map[string]interface{})
	assert.True(ok, "Networks should be a map")
	_, ok = networks["default"].(map[string]interface{})
	assert.True(ok, "Network defaults should be a map")
}

func TestDeleteConfig(t *testing.T) {
	assert := assert.New(t)
	config := make(map[string]interface{})
	err := insertConfig(config, "hello.world", 123)
	assert.NoError(err)
	err = insertConfig(config, "hello.foo.bar", 111)
	assert.NoError(err)
	err = insertConfig(config, "hello.foo.quux", 222)
	assert.NoError(err)

	err = deleteConfig(config, []string{"hello", "world"}, nil)
	assert.NoError(err)
	err = deleteConfig(config, []string{"hello", "foo", "bar"}, nil)
	assert.NoError(err)
	err = deleteConfig(config, []string{"hello", "does", "not", "exist"}, nil)
	assert.IsType(&errConfigNotExist{}, err)

	hello, ok := config["hello"].(map[string]interface{})
	assert.True(ok)
	_, ok = hello["world"]
	assert.False(ok)
	foo, ok := hello["foo"].(map[string]interface{})
	assert.True(ok)
	_, ok = foo["bar"]
	assert.False(ok)
	_, ok = foo["quux"]
	assert.True(ok)
}

func TestConfigMapDifference(t *testing.T) {
	assert := assert.New(t)

	var leftMap map[string]interface{}
	err := json.Unmarshal([]byte(`{
	    "toplevel": "value",
	    "key": {
	        "secondary": "value2",
	        "empty": {
	            "removed": "value3"
	        },
	        "extra": 4
	    },
	    "also_removed": "yes"
	}`), &leftMap)
	assert.NoError(err)

	var rightMap map[interface{}]interface{}
	err = yaml.Unmarshal([]byte(strings.Replace(`
	key:
	    empty:
	        removed: true
	    extra: yes
	    not_in_config: yay
	also_removed: please
	`, "\t", "    ", -1)), &rightMap)
	assert.NoError(err)

	err = configMapDifference(leftMap, rightMap)
	assert.NoError(err)

	var expected map[string]interface{}
	err = json.Unmarshal([]byte(`{
	    "toplevel": "value",
	    "key": {
	        "secondary": "value2",
	        "empty": {}
	    }
	}`), &expected)
	assert.NoError(err)

	assert.Equal(expected, leftMap)
}
