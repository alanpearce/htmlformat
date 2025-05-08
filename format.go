package htmlformat

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Document formats a HTML document.
func Document(w io.Writer, r io.Reader) (err error) {
	node, err := html.Parse(r)
	if err != nil {
		return err
	}
	return Nodes(w, []*html.Node{node})
}

// Fragment formats a fragment of a HTML document.
func Fragment(w io.Writer, r io.Reader) (err error) {
	context := &html.Node{
		Type: html.ElementNode,
	}
	nodes, err := html.ParseFragment(r, context)
	if err != nil {
		return err
	}
	return Nodes(w, nodes)
}

// Nodes formats a slice of HTML nodes.
func Nodes(w io.Writer, nodes []*html.Node) (err error) {
	for _, node := range nodes {
		if err = printNode(w, node, false, 0); err != nil {
			return
		}
	}
	return
}

// Is this node a tag with no end tag such as <meta> or <br>?
// http://www.w3.org/TR/html-markup/syntax.html#syntax-elements
func isVoidElement(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Area, atom.Base, atom.Br, atom.Col, atom.Command, atom.Embed,
		atom.Hr, atom.Img, atom.Input, atom.Keygen, atom.Link,
		atom.Meta, atom.Param, atom.Source, atom.Track, atom.Wbr:
		return true
	}
	return false
}

func isSpecialContentElement(n *html.Node) bool {
	if n != nil {
		switch n.DataAtom {
		case atom.Style,
			atom.Script:
			return true
		}
	}
	return false
}

func isEmptyTextNode(n *html.Node) bool {
	return n.Type == html.TextNode && strings.TrimSpace(n.Data) == ""
}

func collapseWhitespace(in string) string {
	leading := unicode.IsSpace(getFirstRune(in))
	trailing := unicode.IsSpace(getLastRune(in))

	out := strings.TrimSpace(in)
	switch {
	case leading && trailing:
		return " " + out + " "
	case leading:
		return " " + out
	case trailing:
		return out + " "
	default:
		return out
	}
}

func getFirstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func getLastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func hasSingleTextChild(n *html.Node) bool {
	return n != nil && n.FirstChild != nil && n.FirstChild == n.LastChild &&
		n.FirstChild.Type == html.TextNode
}

func printNode(w io.Writer, n *html.Node, pre bool, level int) (err error) {
	switch n.Type {
	case html.TextNode:
		if pre {
			if _, err = fmt.Fprint(w, n.Data); err != nil {
				return
			}
			return nil
		}
		s := n.Data
		s = strings.TrimSpace(s)
		if s != "" {
			if !isSpecialContentElement(n.Parent) && !hasSingleTextChild(n.Parent) &&
				(n.PrevSibling == nil || !unicode.IsPunct(getFirstRune(s))) {
				if err = printIndent(w, level); err != nil {
					return
				}
			}
			if isSpecialContentElement(n.Parent) {
				scanner := bufio.NewScanner(strings.NewReader(s))
				for scanner.Scan() {
					t := scanner.Text()
					if _, err = fmt.Fprintln(w); err != nil {
						return
					}
					if err = printIndent(w, level+1); err != nil {
						return
					}
					if _, err = fmt.Fprint(w, t); err != nil {
						return
					}
				}
				if err = scanner.Err(); err != nil {
					return
				}
				if _, err = fmt.Fprintln(w); err != nil {
					return
				}
			} else {
				if _, err = fmt.Fprint(w, collapseWhitespace(s)); err != nil {
					return
				}
				if !hasSingleTextChild(n.Parent) &&
					(n.NextSibling == nil || !unicode.IsPunct(getLastRune(strings.TrimSpace(s)))) {
					if _, err = fmt.Fprint(w, "\n"); err != nil {
						return
					}
				}
			}
		}
	case html.ElementNode:
		if n.PrevSibling == nil ||
			(n.PrevSibling.Type != html.TextNode || !unicode.IsPunct(getLastRune(strings.TrimSpace(n.PrevSibling.Data)))) {
			if err = printIndent(w, level); err != nil {
				return
			}
		}
		if _, err = fmt.Fprintf(w, "<%s", n.Data); err != nil {
			return
		}
		for _, a := range n.Attr {
			val := html.EscapeString(a.Val)
			if _, err = fmt.Fprintf(w, ` %s="%s"`, a.Key, val); err != nil {
				return
			}
		}
		if _, err = fmt.Fprint(w, ">"); err != nil {
			return
		}
		if !hasSingleTextChild(n) {
			if _, err = fmt.Fprint(w, "\n"); err != nil {
				return
			}
		}
		if !isVoidElement(n) {
			if err = printChildren(w, n, n.Data == "pre", level+1); err != nil {
				return
			}
			if isSpecialContentElement(n) || !hasSingleTextChild(n) {
				if err = printIndent(w, level); err != nil {
					return
				}
			}
			if _, err = fmt.Fprintf(w, "</%s>", n.Data); err != nil {
				return
			}

			if n.NextSibling == nil ||
				(!unicode.IsPunct(getFirstRune(n.NextSibling.Data)) || n.NextSibling.Type == html.ElementNode) {
				if _, err = fmt.Fprint(w, "\n"); err != nil {
					return
				}
			}
		}
	case html.CommentNode:
		if err = printIndent(w, level); err != nil {
			return
		}
		if _, err = fmt.Fprintf(w, "<!--%s-->\n", n.Data); err != nil {
			return
		}
		if err = printChildren(w, n, false, level); err != nil {
			return
		}
	case html.DoctypeNode, html.DocumentNode:
		if err = printChildren(w, n, false, level); err != nil {
			return
		}
	}
	return
}

func printChildren(w io.Writer, n *html.Node, pre bool, level int) (err error) {
	child := n.FirstChild
	for child != nil {
		if err = printNode(w, child, pre, level); err != nil {
			return
		}
		child = child.NextSibling
	}
	return
}

func printIndent(w io.Writer, level int) (err error) {
	_, err = fmt.Fprint(w, strings.Repeat(" ", level))
	return err
}
