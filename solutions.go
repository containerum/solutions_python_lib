package solutions

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"path"
	"strings"
	"text/template"

	"regexp"
	"sync"

	"github.com/tdewolff/minify"
	jmin "github.com/tdewolff/minify/json"
)

const SolutionConfigFile = ".containerum.json"
const NamespaceSelectorKey = "NS_SELECTOR"
const NamespaceKey = "NS"

type SolutionConfig struct {
	Env map[string]interface{} `json:"env"`
	Run []struct {
		Type       string `json:"type"`
		ConfigFile string `json:"config_file"`
	}
}

type Solution struct {
	config *SolutionConfig
	dir    string
	tmpl   *template.Template
	mu     *sync.Mutex
}

type SolutionSequencePart struct {
	Type   string
	Config string
}

func OpenSolution(solutionPath string) (*Solution, error) {
	content, err := ioutil.ReadFile(path.Join(solutionPath, SolutionConfigFile))
	if err != nil {
		return nil, errors.New("cannot open solution config: " + err.Error())
	}

	tmpl := template.New("solution").Funcs(templateFunctions)
	t, err := tmpl.Parse(string(content))
	if err != nil {
		return nil, err
	}

	var parsedConfig bytes.Buffer
	err = t.Execute(&parsedConfig, nil)
	if err != nil {
		return nil, err
	}

	var config SolutionConfig
	err = json.Unmarshal(parsedConfig.Bytes(), &config)

	return &Solution{
		config: &config,
		dir:    solutionPath,
		tmpl:   tmpl,
		mu:     &sync.Mutex{},
	}, err
}

func (s *Solution) SetTemplateFunction(templateFunctionName string, function interface{}) {
	s.mu.Lock()
	s.tmpl = s.tmpl.Funcs(template.FuncMap{templateFunctionName: function})
	s.mu.Unlock()
}

func (s *Solution) AddTemplateFunctions(functions template.FuncMap) {
	s.mu.Lock()
	s.tmpl = s.tmpl.Funcs(functions)
	s.mu.Unlock()
}

func (s *Solution) SetValue(key string, value interface{}) {
	s.mu.Lock()
	s.config.Env[key] = value
	s.mu.Unlock()
}

func (s *Solution) AddValues(kv map[string]interface{}) {
	s.mu.Lock()
	for k, v := range kv {
		s.config.Env[k] = v
	}
	s.mu.Unlock()
}

func (s *Solution) GenerateRunSequence(namespace string) (ret []SolutionSequencePart, err error) {
	var errs []string

	s.mu.Lock()
	env := s.config.Env
	s.mu.Unlock()

	env[NamespaceKey] = namespace
	env[NamespaceSelectorKey] = NamespaceSelector(namespace)

	for _, v := range s.config.Run {
		cfg, err := ioutil.ReadFile(path.Join(s.dir, v.ConfigFile))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		s.mu.Lock()
		tmpl, err := s.tmpl.Parse(string(cfg))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, env)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		s.mu.Unlock()

		ret = append(ret, SolutionSequencePart{
			Type:   v.Type,
			Config: minifyJson(buf.String()),
		})
	}

	if len(errs) != 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}

	return ret, nil
}

var m = minify.New()

func init() {
	m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), jmin.Minify)
}

func minifyJson(in string) string {
	ret, _ := m.String("text/json", in)
	return ret
}
