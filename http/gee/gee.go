package gee

import (
	"html/template"
	"net/http"
	"path"
	"strings"
)

type HandlerFunc func(ctx *Context)

type RouterGroup struct {
	prefix      string
	middlewares []HandlerFunc
	parent      *RouterGroup
	engine      *Engine
}

type Engine struct {
	*RouterGroup
	router        *router
	groups        []*RouterGroup
	htmlTemplates *template.Template
	funcMap       template.FuncMap
}

func (e *Engine) SetFuncMap(funcMap template.FuncMap) {
	e.funcMap = funcMap
}
func (e *Engine) LoadHTMLGlob(pattern string) {
	e.htmlTemplates = template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern))
}

// 处理请求
func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc
	// 匹配路径和已有的分组，如果前缀匹配，注册对应的中间件
	for _, group := range e.groups {
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}
	// 一个请求生成一个context结构
	c := newContext(w, req)
	c.engine = e
	c.handlers = middlewares
	e.router.handler(c)
}

func New() *Engine {
	e := &Engine{router: newRouter()}
	e.RouterGroup = &RouterGroup{engine: e}
	e.groups = []*RouterGroup{
		e.RouterGroup,
	}
	return e
}

// Group 生成子分组
func (g *RouterGroup) Group(prefix string) *RouterGroup {
	// 结构构建，加上结构本身的前缀
	group := &RouterGroup{
		engine: g.engine,
		prefix: g.prefix + prefix,
		parent: g,
	}
	// engine需要记录所有的分组，方便匹配和加载中间件
	g.engine.groups = append(g.engine.groups, group)
	return group
}

// Use 加载中间件
func (g *RouterGroup) Use(middlewares ...HandlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// 新增路由
func (g *RouterGroup) addRouter(method string, pattern string, handler HandlerFunc) {
	pattern = g.prefix + pattern
	g.engine.router.addRoute(method, pattern, handler)
}
func (g *RouterGroup) GET(pattern string, handlerFunc HandlerFunc) {
	g.addRouter("GET", pattern, handlerFunc)
}
func (g *RouterGroup) POST(pattern string, handlerFunc HandlerFunc) {
	g.addRouter("POST", pattern, handlerFunc)
}

func (g *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	absolutePath := path.Join(g.prefix, relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
	return func(c *Context) {
		file := c.Param("filePath")
		if _, err := fs.Open(file); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Req)
	}
}
func (g *RouterGroup) Static(relativePath string, root string) {
	handler := g.createStaticHandler(relativePath, http.Dir(root))
	pattern := path.Join(relativePath, "/*filePath")
	g.GET(pattern, handler)
}

func (e *Engine) RUN(addr string) {
	http.ListenAndServe(addr, e)
}
