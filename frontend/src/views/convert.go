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
var htmlTemplateFolder = "backbone/templates/"
var jsComponentFolder = "backbone/views/"
var targetVueFolder = "src/views_draft"

type EventHandler struct {
	Raw         string
	EventName   string
	JsClassName string
	HandlerName string
}

type ComponentInfo struct {
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

	// get class name
	quoteChars := "'\"`"
	r, _ := regexp.Compile(`\n\s*className\(\s*\)\s*{\s*\n\s*return\s+[` + quoteChars + `](.*?)[` + quoteChars + `][;]?\s*\n`)
	matches := r.FindStringSubmatch(string(contentBytes))
	if len(matches) > 0 {
		componentInfo.ClassName = matches[1]
	}

	// get event handlers
	r, _ = regexp.Compile(`\s*events\(\s*\)\s*{\s*\n\s*return\s+{\n((\s*(\S+)\s+(\S+)': '(\S+)\s*\n)+)\s*};`)
	matches = r.FindStringSubmatch(string(contentBytes))
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

func applyEventHandlerToTemplate(templateFileContent string, jsFileContent string, componentInfo ComponentInfo) (string, string) {

	for _, info := range componentInfo.EventHanders {
		m := regexp.MustCompile(`( class="(?:.*?\s)?)` + info.JsClassName + `((?:\s.*?)?")`)

		isIDMatch := false
		if strings.HasPrefix(info.JsClassName, "#") {
			isIDMatch = true

			m = regexp.MustCompile(`( id="` + info.JsClassName[1:] + `")`)
		}

		eventName := ""
		switch info.EventName {
		case "click":
			eventName = "click"
		case "keydown":
			eventName = "on-keydown"
		case "keyup":
			eventName = "on-keyup"
		case "change":
			eventName = "on-change"
		case "focus":
			eventName = "on-focus"
		case "mouseleave":
			eventName = "on-mouseleave"
		}

		if len(eventName) > 0 {
			Str := "$1$2 @" + eventName + "=\"" + info.HandlerName + "\" "
			if isIDMatch {
				Str = "$1 @" + eventName + "=\"" + info.HandlerName + "\" "
			}

			if m.Match([]byte(templateFileContent)) {
				templateFileContent = m.ReplaceAllString(templateFileContent, Str)

				jsFileContent = strings.ReplaceAll(jsFileContent, info.Raw+",\n", "")
				jsFileContent = strings.ReplaceAll(jsFileContent, info.Raw+"\n", "")
			}
		}

		eventNames[info.EventName] = true
	}

	return templateFileContent, jsFileContent
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
	Str = " :hidden=\"$1\""
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
	Str = "${1}<div v-if=\"${2}\">\n"
	content = m.ReplaceAllString(content, Str)

	// update: <% } else if (ob.listing.shippingOptions) { %>
	m = regexp.MustCompile(`(\s*)<%\s*\}\s*else if\s*\((.+?)\)\s*\{\s*(%>)?\s*\n`)
	Str = "${1}</div>\n${1}<div v-else-if=\"${2}\">\n"
	content = m.ReplaceAllString(content, Str)

	// update: <% } else { %>
	m = regexp.MustCompile(`(\s*)<%\s*}\s*else\s*\{\s*%>\s*\n`)
	Str = "${1}</div>\n${1}<div v-else>\n"
	content = m.ReplaceAllString(content, Str)

	// update if/else if/else close tag: <% } %>
	m = regexp.MustCompile(`(\s*)<%\s*}\s*%>\s*\n`)
	Str = "${1}</div>\n"
	content = m.ReplaceAllString(content, Str)

	// update <% ob.coupons.forEach((coupon) => { %>
	m = regexp.MustCompile(`(\s*)<%\s*(\S.*\S)\.forEach\(\((\w+)\)\s*=>\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<div v-for=\"(${3}, j) in ${2}\" :key=\"j\">\n"
	content = m.ReplaceAllString(content, Str)

	// update <% ob.coupons.forEach(coupon => { %>
	m = regexp.MustCompile(`(\s*)<%\s*(\S.*\S)\.forEach\((\w+)\s*=>\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<div v-for=\"(${3}, j) in ${2}\" :key=\"j\">\n"
	content = m.ReplaceAllString(content, Str)

	// update <% ob.coupons.forEach((coupon, i) => { %>
	m = regexp.MustCompile(`(\s*)<%\s*(\S.*\S)\.forEach\(\((\w+, (\w+))\)\s*=>\s*\{\s*(%>)?\s*\n`)
	Str = "${1}<div v-for=\"(${3}) in ${2}\" :key=\"${4}\">\n"
	content = m.ReplaceAllString(content, Str)

	// update forEach close tag: <% }); %>
	m = regexp.MustCompile(`(\s*)<%\s*}\);\s*%>\s*\n`)
	Str = "${1}</div>\n"
	content = m.ReplaceAllString(content, Str)

	return content
}

func updateJsFileContent(content string) string {
	// Update function definition
	m := regexp.MustCompile(`\n(\s+)((\w+)\(.*\) \{)\n`)
	Str := "\n${1}function $2\n"
	content = m.ReplaceAllString(content, Str)

	return strings.ReplaceAll(content, " constructor(", " loadData(")
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
			jsFileContent := updateJsFileContent(string(jsFileBytes))

			templateFileContent, jsFileContent = applyEventHandlerToTemplate(templateFileContent, jsFileContent, componentInfo)

			rootDivTag := "  <div>\n"
			if len(componentInfo.ClassName) > 0 {
				rootDivTag = `  <div class="` + componentInfo.ClassName + "\">\n"
			}

			text := "<template>\n" + rootDivTag + templateFileContent + `
  </div>
</template>
  
<script setup>
const props = defineProps({
  phase: String,
  outdatedHash: String,
})

loadData(props);

render();

` + jsFileContent + `
</script>
<style lang="scss" scoped>
</style>

`
			os.WriteFile(path.Join(dir, componentName+".vue"), []byte(text), fs.ModePerm)
		}
	}
	return nil
}

func main() {
	filepath.WalkDir(path.Join(baseDir, htmlTemplateFolder), walk)

	fmt.Println("events are: ")
	for key := range eventNames {
		fmt.Println(key)
	}
}
