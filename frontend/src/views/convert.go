package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var baseDir = "/Users/mingfeng/dev/openbazaar/openbazaar-desktop/frontend"
var htmlTemplateFolder = "backbone/templates/modals/onboarding"
var jsComponentFolder = "backbone/views/modals/onboarding"

type EventHandler struct {
	Raw         string
	EventName   string
	JsClassName string
	HandlerName string
}

type ComponentInfo struct {
	Name         string
	IsModal      bool
	TagName      string
	ClassName    string
	EventHanders []EventHandler
}

func readJsFileContent(templateFilePath string, name string) ([]byte, ComponentInfo, error) {
	componentInfo := ComponentInfo{}

	dir := filepath.Dir(templateFilePath)
	jsDir := strings.ReplaceAll(dir, path.Join(baseDir, htmlTemplateFolder), path.Join(baseDir, jsComponentFolder))

	contentBytes, err := os.ReadFile(path.Join(jsDir, name))
	if err != nil {
		return contentBytes, ComponentInfo{}, err
	}

	content := string(contentBytes)
	componentInfo.IsModal = strings.Contains(content, "export default class extends BaseModal")

	// get class name
	quoteChars := "'\"`"
	r, _ := regexp.Compile(`\n\s*(?:get\s*)?className\(\s*\)\s*{\s*\n\s*return\s+[` + quoteChars + `](.*?)[` + quoteChars + `][;]?\s*\n`)
	matches := r.FindStringSubmatch(content)
	if len(matches) > 0 {
		componentInfo.ClassName = matches[1]
	}

	// get tag name
	r, _ = regexp.Compile(`\n\s*(?:get\s*)?tagName\(\s*\)\s*{\s*\n\s*return\s+[` + quoteChars + `](.*?)[` + quoteChars + `][;]?\s*\n`)
	matches = r.FindStringSubmatch(content)
	if len(matches) > 0 {
		componentInfo.TagName = matches[1]
	}

	// get event handlers
	r, _ = regexp.Compile(`\s*events\(\s*\)\s*{\s*\n\s*return\s+{\n((.*\n)+)\s*};`)
	matches = r.FindStringSubmatch(content)
	if len(matches) > 0 {
		handlersStr := matches[1]
		fmt.Println(handlersStr)

		r, _ = regexp.Compile(`'(\S+)\s+(\S+)': '(\S+)'`)
		allMatches := r.FindAllStringSubmatch(handlersStr, -1)
		for _, match := range allMatches {
			componentInfo.EventHanders = append(componentInfo.EventHanders, EventHandler{
				Raw:         match[0],
				EventName:   match[1],
				JsClassName: strings.TrimPrefix(match[2], "."),
				HandlerName: match[3],
			})
		}
	}

	return contentBytes, componentInfo, nil
}

var eventNames = map[string]bool{}

func applyEventHandlerToTemplate(templateFileContent string, jsMethodsListContent string, componentInfo ComponentInfo) (string, string) {

	for _, info := range componentInfo.EventHanders {
		m := regexp.MustCompile(`( class="(?:.*?\s)?)` + info.JsClassName + `((?:\s.*?)?")`)

		isIDMatch := false
		if strings.HasPrefix(info.JsClassName, "#") {
			isIDMatch = true

			m = regexp.MustCompile(`(\s+id="` + info.JsClassName[1:] + `")`)
		}

		eventName := ""
		switch info.EventName {
		case "click":
			eventName = "click"
		case "keydown":
			eventName = "keydown"
		case "keyup":
			eventName = "keyup"
		case "change":
			eventName = "change"
		case "focus":
			eventName = "focus"
		case "mouseleave":
			eventName = "mouseleave"
		}

		if len(eventName) > 0 {
			Str := "$1$2 @" + eventName + "=\"" + info.HandlerName + "\" "
			if isIDMatch {
				Str = "$1 @" + eventName + "=\"" + info.HandlerName + "\" "
			}

			if m.Match([]byte(templateFileContent)) {
				templateFileContent = m.ReplaceAllString(templateFileContent, Str)

				jsMethodsListContent = strings.ReplaceAll(jsMethodsListContent, info.Raw+",\n", "")
				jsMethodsListContent = strings.ReplaceAll(jsMethodsListContent, info.Raw+"\n", "")
			}
		}

		eventNames[info.EventName] = true
	}

	return templateFileContent, jsMethodsListContent
}

func updateTemplateContent(content string) string {
	// Update style like: class="<%= ob.wrappingClass %>"
	m := regexp.MustCompile(` (\S+)="<%=\s*(.+?)\s*%>"`)
	Str := " :${1}=\"$2\""
	content = m.ReplaceAllString(content, Str)

	// href="https://www.facebook.com/sharer/sharer.php?u=<%= shareURL %>"
	m = regexp.MustCompile(` (\S+)="([^"]*)<%=\s*(.+?)\s*%>([^"]*)"`)
	Str = " :${1}=\"`$2${$3}$4`\""
	content = m.ReplaceAllString(content, Str)

	// <% if (cur === ob.cryptoAmountCurrency) print('selected'); %>
	m = regexp.MustCompile(` <% if \((.*?)\) print\('(selected|disabled|checked|required)'\)(;?)\s*%>`)
	Str = " :$2=\"$1\""
	content = m.ReplaceAllString(content, Str)

	// <% if (cur === ob.cryptoAmountCurrency) print('hide'); %>
	m = regexp.MustCompile(` <% if \((.*?)\) print\('(hide)'\)(;?)\s*%>`)
	Str = " v-show=\"!$1\""
	content = m.ReplaceAllString(content, Str)

	//  class="abc <% if (cur === ob.cryptoAmountCurrency) print('active')" %>
	m = regexp.MustCompile(` (\S+)="([^"]*) <% if \((.*?)\) print\('(\S+)'\)(;?)\s+%>(.*?)"`)
	Str = " :$1=\"`$2 ${$3 ? '$4' : ''}$5`\""
	content = m.ReplaceAllString(content, Str)

	// maxlength=<%= ob.itemConstraints.maxPaymentAddressLength %>
	m = regexp.MustCompile(` maxlength=<%=\s*(.*?)\s*%>`)
	Str = " :maxlength=\"$1\""
	content = m.ReplaceAllString(content, Str)

	// update variable
	m = regexp.MustCompile(`<%=\s*(.+?)\s*%>`)
	Str = "{{ ${1} }}"
	content = m.ReplaceAllString(content, Str)

	// update: <% if (cur.disabled && ob.disabledMsg) { %>
	m = regexp.MustCompile(`(\s*)<%\s*if\s*\((.+?)\)\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<template v-if=\"${2}\">\n"
	content = m.ReplaceAllString(content, Str)

	// update: <% } else if (ob.listing.shippingOptions) { %>
	m = regexp.MustCompile(`(\s*)<%\s*\}\s*else if\s*\((.+?)\)\s*\{\s*(%>)?\s*\n`)
	Str = "${1}</template>\n${1}<template v-else-if=\"${2}\">\n"
	content = m.ReplaceAllString(content, Str)

	// update: <% } else { %>
	m = regexp.MustCompile(`(\s*)<%\s*}\s*else\s*\{\s*%>\s*\n`)
	Str = "${1}</template>\n${1}<template v-else>\n"
	content = m.ReplaceAllString(content, Str)

	// update if/else if/else close tag: <% } %>
	m = regexp.MustCompile(`(\s*)<%\s*}\s*%>\s*\n`)
	Str = "${1}</template>\n"
	content = m.ReplaceAllString(content, Str)

	// update if/else if/else close tag: <% } %>
	m = regexp.MustCompile(`(\s*)<%\s*}\s*%>\s*$`)
	Str = "${1}</template>\n"
	content = m.ReplaceAllString(content, Str)

	// update <% ob.coupons.forEach((coupon) => { %>
	m = regexp.MustCompile(`(\s*)<%\s*(\S.*\S)\.forEach\(\((\w+)\)\s*=>\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<template v-for=\"(${3}, j) in ${2}\" :key=\"j\">\n"
	content = m.ReplaceAllString(content, Str)

	// update <% ob.coupons.forEach(coupon => { %>
	m = regexp.MustCompile(`(\s*)<%\s*(\S.*\S)\.forEach\((\w+)\s*=>\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<template v-for=\"(${3}, j) in ${2}\" :key=\"j\">\n"
	content = m.ReplaceAllString(content, Str)

	// update <% ob.coupons.forEach((coupon, i) => { %>
	m = regexp.MustCompile(`(\s*)<%\s*(\S.*\S)\.forEach\(\((\w+, (\w+))\)\s*=>\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<template v-for=\"(${3}) in ${2}\" :key=\"${4}\">\n"
	content = m.ReplaceAllString(content, Str)

	// update forEach close tag: <% }); %>
	m = regexp.MustCompile(`(\s*)<%\s*}\);?\s*%>\s*\n`)
	Str = "${1}</template>\n"
	content = m.ReplaceAllString(content, Str)

	// update forEach close tag: <% }); %>
	m = regexp.MustCompile(`(\s*)<%\s*}\);?\s*%>\s*$`)
	Str = "${1}</template>\n"
	content = m.ReplaceAllString(content, Str)

	return content
}

func updateJsFileContent(content string) (string, string) {
	header := ""

	// // Update function definition
	// m := regexp.MustCompile(`\n(\s+)((\w+)\(.*\) \{)\n`)
	// Str := "\n${1}function $2\n"
	// contentTemp := m.ReplaceAllString(content, Str)

	// methodList := []string{}
	// allMatches := m.FindAllStringSubmatch(content, -1)
	// for _, match := range allMatches {
	// 	methodList = append(methodList, match[3])
	// }

	// content = contentTemp
	// for _, method := range methodList {
	// 	fmt.Printf("method: %s\n", "this."+method+"(")
	// 	content = strings.ReplaceAll(content, "this."+method+"(", method+"(")
	// }

	// Add "," after method
	m := regexp.MustCompile(`\}(\n*(\s+)((\w+)\(.*\) \{)\n)`)
	content = m.ReplaceAllString(content, "},$1")

	result := strings.Split(content, "export default class")
	if len(result) > 0 {
		header = result[0]
		content = strings.ReplaceAll(content, header, "")
	}

	// Remove line "export default class extends baseVw" { and ending }
	m = regexp.MustCompile(`export default class.*\n`)
	content = m.ReplaceAllString(content, "")

	m = regexp.MustCompile(`\n}\n*$`)
	content = m.ReplaceAllString(content, "\n")

	m = regexp.MustCompile(`this.\$\(`)
	content = m.ReplaceAllString(content, "$(")

	// super(opts);
	m = regexp.MustCompile(`super\((.*)\);`)
	content = m.ReplaceAllString(content, "this.baseInit($1);")

	return header, strings.ReplaceAll(content, " constructor(", " loadData(")
}

type TemplateVariable struct {
	Raw string
	Key string
	Val string
}

func applyVarsToTemplate(templateFileContent string, jsMethodsListContent string) (string, string) {
	vars, _ := getTemplateVars(jsMethodsListContent)

	for _, item := range vars {
		m := regexp.MustCompile(`(\W)ob\.` + item.Key + `(\W)`)
		if m.Match([]byte(templateFileContent)) {
			// jsMethodsListContent = strings.ReplaceAll(jsMethodsListContent, item.Raw, "")

			templateFileContent = m.ReplaceAllString(templateFileContent, "${1}params."+item.Key+"${2}")
		}
	}

	return templateFileContent, jsMethodsListContent
}

func getTemplateVars(jsContent string) ([]TemplateVariable, string) {
	vars := []TemplateVariable{}

	r := regexp.MustCompile(`\n\s*this.\$el.html\(t\(\{((.*\n)+)\s*\}\)\)\;`)
	matches := r.FindStringSubmatch(jsContent)
	mappingContent := ""
	if len(matches) > 0 {
		mappingContent = matches[1]
	}

	if len(mappingContent) > 0 {
		r = regexp.MustCompile(`(\s*(\w+):\s*(.*),\s*\n)`)
		allMatches := r.FindAllStringSubmatch(mappingContent, -1)
		for _, match := range allMatches {
			vars = append(vars, TemplateVariable{
				Raw: match[1],
				Key: match[2],
				Val: strings.ReplaceAll(match[3], "this.", ""),
			})
		}
	}

	return vars, mappingContent
}

func capitalize(str string) string {
	bs := []byte(str)
	if len(bs) == 0 {
		return ""
	}
	if bs[0] >= 97 {
		bs[0] = byte(bs[0] - 32)
	}

	return string(bs)
}

func walk(s string, d fs.DirEntry, err error) error {
	libRegEx, e := regexp.Compile(`.(html)$`)
	if e != nil {
		log.Fatal(e)
	}

	if !d.IsDir() {
		if libRegEx.MatchString(d.Name()) {
			dir := filepath.Dir(s)
			dir = strings.ReplaceAll(dir, path.Join(baseDir, htmlTemplateFolder), path.Join(baseDir, htmlTemplateFolder))

			// if !strings.HasSuffix(dir, "summaryTab") {
			// 	return nil
			// }

			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				log.Println(err)
			}

			componentName := strings.ReplaceAll(d.Name(), ".html", "")
			componentName = capitalize(componentName)

			templateFileBytes, err := os.ReadFile(s)
			if err != nil {
				log.Fatal(err)
			}
			templateFileContent := updateTemplateContent(string(templateFileBytes))

			jsFileBytes, componentInfo, err := readJsFileContent(s, componentName+".js")
			if err != nil {
				fmt.Printf("Error: %v\n", strings.ReplaceAll(err.Error(), path.Join(baseDir, jsComponentFolder), ""))
			}
			header, jsMethodsListContent := updateJsFileContent(string(jsFileBytes))

			templateFileContent, jsMethodsListContent = applyEventHandlerToTemplate(templateFileContent, jsMethodsListContent, componentInfo)
			componentInfo.Name = componentName

			// we continue to use ob, no need to change
			// templateFileContent, jsMethodsListContent = applyVarsToTemplate(templateFileContent, jsMethodsListContent)

			_, params := getTemplateVars(jsMethodsListContent)
			if len(params) > 0 {
				params = `
    ob () {
      return {
        ...this.templateHelpers,` + params +
					`      };
    }`
			}

			rootTagName := "div"
			if len(componentInfo.TagName) > 0 {
				rootTagName = componentInfo.TagName
			}

			rootTag := fmt.Sprintf("  <%s>\n", rootTagName)
			if len(componentInfo.ClassName) > 0 {
				rootTag = fmt.Sprintf("  <%s class=\"%s\">\n", rootTagName, componentInfo.ClassName)
			}
			fmt.Printf("rootTag: %s", rootTag)

			endingRootTag := fmt.Sprintf("\n  </%s>", rootTagName)

			if componentInfo.IsModal {
				templateFileContent = buildModalComponent(templateFileContent)
			}
			text := "<template>\n" + rootTag + templateFileContent + endingRootTag + `
</template>

<script>
` + header + `
export default {
  props: {
    options: {
      type: Object,
      default: {},
    },
    bb: Function,
  },
  data () {
    return {
    };
  },
  created () {
    this.initEventChain();

    this.loadData(this.options);
  },
  mounted () {
    this.render();
  },
  computed: {` + params +
				`
  },
	methods: {
` + jsMethodsListContent + `
  }
}
</script>
<style lang="scss" scoped>
</style>
`
			os.WriteFile(path.Join(dir, componentInfo.Name+".vue"), []byte(text), fs.ModePerm)
		}
	}
	return nil
}

func buildModalComponent(templateFileContent string) string {
	return fmt.Sprintf(`    <BaseModal>
      <template v-slot:component>
        %s
      </template>
    </BaseModal>`, templateFileContent)
}

func main() {
	filepath.WalkDir(path.Join(baseDir, htmlTemplateFolder), walk)

	fmt.Println("events are: ")
	for key := range eventNames {
		fmt.Println(key)
	}
}
