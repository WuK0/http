# http
http轮子学习模拟

本仓库代码参考模拟自[7天用Go从零实现Web框架Gee教程 | 极客兔兔 (geektutu.com)](https://geektutu.com/post/gee.html)，主要参考学习了golang http框架Gin进行轮子的复现，对于理解路由转发和中间件处理机制有一定的帮助。

# 核心代码解析

## gee.go
### 路由分组结构 RouterGroup
Engine也是一个大的RouterGroup，提高灵活性
```go
type RouterGroup struct {
	prefix string
	middlewares []HandlerFunc
	parent *RouterGroup
	engine *Engine
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
func (g *RouterGroup) addRouter(method string, pattern string, handler HandlerFunc)  {
	pattern = g.prefix + pattern
	g.engine.router.addRoute(method, pattern, handler)
}
func (g *RouterGroup) GET(pattern string, handlerFunc HandlerFunc) {
	g.addRouter("GET", pattern, handlerFunc)
}
func (g *RouterGroup) POST(pattern string, handlerFunc HandlerFunc) {
	g.addRouter("POST", pattern, handlerFunc)
}

func (g *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc{
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
func (g *RouterGroup) Static(relativePath string, root string)  {
	handler := g.createStaticHandler(relativePath, http.Dir(root))
	pattern := path.Join(relativePath, "/*filePath")
	g.GET(pattern, handler)
}
```
### engine结构
```go
type Engine struct {
	*RouterGroup
	router *router
	groups []*RouterGroup
	htmlTemplates *template.Template
	funcMap template.FuncMap
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
	// 新建路由
    e := &Engine{router: newRouter()}
    e.RouterGroup = &RouterGroup{engine: e}
    e.groups = []*RouterGroup{
        e.RouterGroup,
    }
    return e
}
// 本质上还是调用了原生的http包
func (e *Engine) RUN(addr string) {
    http.ListenAndServe(addr, e)
}

```
## router.go
路由结构，即基于该结构可以找到对应请求路径所注册的处理方法
```go
type router struct {
	// 一种请求方法对应一棵前缀树
	roots map[string]*node
	// 一种路由匹配一个请求方法
	handlers map[string]HandlerFunc
}

func newRouter() *router {
	return &router{
		roots: make(map[string]*node),
		handlers: make(map[string]HandlerFunc),
	}
}

// 新增路由
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	log.Printf("Route %4s - %s", method, pattern)
	parts := parsePattern(pattern)
	_, ok := r.roots[method]
	if !ok {
		r.roots[method] = &node{}
	}
	r.roots[method].insert(pattern, parts, 0)
	// 通过请求方式进行区分
	key := method + "-" + pattern
	r.handlers[key] = handler
}

// 匹配路由
func (r *router) getRoute(method, path string) (*node, map[string]string) {
	root, ok := r.roots[method]
	if !ok {
		return nil, nil
	}
	params := make(map[string]string)
	searchParts := parsePattern(path)
	// 先查路由数是否有相应结构的节点
	n := root.search(searchParts, 0)
	if n != nil {
		// 取出节点的pattern，做参数解析（restful路劲中的参数对应）
		parts := parsePattern(n.pattern)
		for i, v := range parts {
			if v[0] == ':' {
				params[v[1:]] = searchParts[i]
			}
			if v[0] == '*' && len(v) > 1{
				params[v[1:]] = strings.Join(searchParts[i:], "/")
				break
			}
		}
		return n, params
	}
	return nil, nil
}

// 找到路由和对应的处理方法，加入context中，结合中间件一起执行
func (r *router) handler(c *Context) {
	n, params := r.getRoute(c.Method, c.Path)
	if n != nil {
		c.Params = params
		key := c.Method + "-" + n.pattern
		c.handlers = append(c.handlers, r.handlers[key])
	}else {
		c.handlers = append(c.handlers, func(c *Context) {
			c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
		})
	}
	c.Next()
}

// 请求路径/pattern预处理
func parsePattern(pattern string) []string {
	vs := strings.Split(pattern, "/")
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		if v != "" {
			parts = append(parts, v)
			if v[0] == '*' {
				break
			}
		}
	}
	return parts
}
```
## trie.go
前缀树数据结构，方便寻找路由结构，同时适配restful请求
```go
import "strings"

// pattern字段有值的节点才是真正的路由节点，part节点仅是表示当前节点的路径
type node struct {
	pattern string
	part string
	// 如果part是':'或'*'开头，则为true，表示皆可匹配
	isWild bool
	children []*node
}

// 匹配孩子
func (n *node) matchChild(part string) *node {
	for _, child := range n.children {
		if child.part == part || child.isWild {
			return child
		}
	}
	return nil
}
// 批量匹配孩子，用户搜索
func (n *node) matchChildren(part string) []*node {
	res := make([]*node, 0)
	for _, child := range n.children {
		if child.part == part || child.isWild {
			res = append(res, child)
		}
	}
	return res
}

// 新增节点
func (n *node) insert(pattern string, parts []string, height int)  {
	if len(parts) == height {
		// 找到最终的路由节点，填充pattern
		n.pattern = pattern
		return
	}
	part := parts[height]
	child := n.matchChild(part)
	if child == nil {
		// 没有节点则新增
		child = &node{
			part: part,
			isWild: part[0] == ':' || part[0] == '*',
		}
		n.children = append(n.children, child)
	}
	child.insert(pattern, parts, height + 1)
}

// 搜索节点
func (n *node) search(parts []string, height int) *node {
	if height == len(parts) || strings.HasPrefix(n.part, "*"){
		if n.pattern == "" {
			// 匹配到最后，如果pattern没有值，证明之前没插入过该节点
			return nil
		}
		return n
	}
	part := parts[height]
	for _, child := range n.matchChildren(part) {
		result := child.search(parts, height + 1)
		if result != nil {
			return result
		}
	}
	return nil
}
```
## context.go
Context结构，在请求进来时新建context结构，定义了JSON，String，Data等多种输出方式，响应请求
```go
type Context struct {
	// origin object
	Writer http.ResponseWriter
	Req    *http.Request
	// request info
	Path   string
	Method string
	Params map[string]string
	// response info
	StatusCode int
	// middleware
	handlers []HandlerFunc
	index int
	// engine
	engine *Engine
}
```
关键的代码：中间件加载和洋葱运行，维护一个全局的index变量，起到一个递归回调的“洋葱”效果
```go
func (c *Context) Next()  {
	c.index++
	for ; c.index < len(c.handlers); c.index++{
		c.handlers[c.index](c)
	}
}
```