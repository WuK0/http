package gee

import "strings"

// pattern字段有值的节点才是真正的路由节点，part节点仅是表示当前节点的路径
type node struct {
	pattern string
	part    string
	// 如果part是':'或'*'开头，则为true，表示皆可匹配
	isWild   bool
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
func (n *node) insert(pattern string, parts []string, height int) {
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
			part:   part,
			isWild: part[0] == ':' || part[0] == '*',
		}
		n.children = append(n.children, child)
	}
	child.insert(pattern, parts, height+1)
}

// 搜索节点
func (n *node) search(parts []string, height int) *node {
	if height == len(parts) || strings.HasPrefix(n.part, "*") {
		if n.pattern == "" {
			// 匹配到最后，如果pattern没有值，证明之前没插入过该节点
			return nil
		}
		return n
	}
	part := parts[height]
	for _, child := range n.matchChildren(part) {
		result := child.search(parts, height+1)
		if result != nil {
			return result
		}
	}
	return nil
}
