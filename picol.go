package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var fname = flag.String("f", "", "file name")

func main() {
	flag.Parse()
	interp := InitInterp()
	interp.RegisterCoreCommands()

	buf, err := ioutil.ReadFile(*fname)
	if err == nil {
		retcode := interp.Eval(string(buf))
		if retcode != PICOL_OK {
			fmt.Printf("[%d] %s\n", retcode, interp.result)
		}
	} else {
		for {
			fmt.Print("picol> ")
			//clibuf := bufio.ReadLine()
			scanner := bufio.NewReader(os.Stdin)
			clibuf, _ := scanner.ReadString('\n')
			retcode := interp.Eval(clibuf[:len(clibuf)-1])
			if len(interp.result) != 0 {
				fmt.Printf("[%d] %s\n", retcode, interp.result)
			}
		}
	}

	// TODO from file
	/*} else if (argc == 2) {
	    char buf[1024*16];
	    FILE *fp = fopen(argv[1],"r");
	    if (!fp) {
	        perror("open"); exit(1);
	    }
	    buf[fread(buf,1,1024*16,fp)] = '\0';
	    fclose(fp);
	    if (picolEval(&interp,buf) != PICOL_OK) printf("%s\n", interp.result);
	}*/
}

const (
	PICOL_OK = iota
	PICOL_ERR
	PICOL_RETURN
	PICOL_BREAK
	PICOL_CONTINUE

	PT_ESC = iota
	PT_STR
	PT_CMD
	PT_VAR
	PT_SEP
	PT_EOL
	PT_EOF
)

type picolParser struct {
	text              string
	p, start, end, ln int
	insidequote       int
	type_             int

	//char *text
	//char *p /* current text position */
	//int len /* remaining length */
	//char *start /* token start */
	//char *end /* token end */
	//int type /* token type, PT_... */
	//int insidequote /* True if inside " " */
}

func picolInitParser(text string) *picolParser {
	return &picolParser{text, 0, 0, 0, len(text), 0, PT_EOL}
	/*
	   p->text = p->p = text;
	   p->len = strlen(text);
	   p->start = 0; p->end = 0; p->insidequote = 0;
	   p->type = PT_EOL;
	*/
}

func (p *picolParser) picolParseSep() int {
	p.start = p.p
	c := p.text[p.p]
	for ; c == ' ' || c == '\t' || c == '\n' || c == '\r'; c = p.text[p.p] {
		p.p++
		p.ln--
		if p.p == len(p.text) {
			break
		}
	}
	p.end = p.p - 1
	p.type_ = PT_SEP
	return PICOL_OK

	/*
	   p->start = p->p;
	   while(*p->p == ' ' || *p->p == '\t' || *p->p == '\n' || *p->p == '\r') {
	       p->p++; p->len--;
	   }
	   p->end = p->p-1;
	   p->type = PT_SEP;
	*/
}

func (p *picolParser) picolParseEol() int {
	// XXX add ';' and PT_EOL
	p.start = p.p

	c := p.text[p.p]
	for ; c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ';'; c = p.text[p.p] {
		p.p++
		p.ln--
		if p.p == len(p.text) {
			break
		}
	}
	p.end = p.p - 1
	p.type_ = PT_EOL
	return PICOL_OK

	/*
	   p->start = p->p;
	   while(*p->p == ' ' || *p->p == '\t' || *p->p == '\n' || *p->p == '\r' ||
	         *p->p == ';')
	   {
	       p->p++; p->len--;
	   }
	   p->end = p->p-1;
	   p->type = PT_EOL;
	*/
	//return PICOL_OK
}

func (p *picolParser) picolParseCommand() int {
	level, blevel := 1, 0

	p.p++
	p.ln--
	p.start = p.p

Loop:
	for {
		switch {
		case p.ln == 0 || len(p.text)-1 >= p.p:
			break Loop
		case p.text[p.p] == '[' && blevel == 0:
			level++
		case p.text[p.p] == ']' && blevel == 0:
			level--
			if level == 0 {
				break Loop
			}
		case p.text[p.p] == '\\':
			p.p++
			p.ln--

		case p.text[p.p] == '{':
			blevel++
		case p.text[p.p] == '}':
			if blevel != 0 {
				blevel--
			}
		}
		p.p++
		p.ln--
	}
	p.end = p.p - 1
	p.type_ = PT_CMD
	if p.text[p.p] == ']' {
		p.p++
		p.ln--
	}
	return PICOL_OK

	/*
	   int level = 1;
	   int blevel = 0;
	   p->start = ++p->p; p->len--;
	   while (1) {
	       if (p->len == 0) {
	           break;
	       } else if (*p->p == '[' && blevel == 0) {
	           level++;
	       } else if (*p->p == ']' && blevel == 0) {
	           if (!--level) break;
	       } else if (*p->p == '\\') {
	           p->p++; p->len--;
	       } else if (*p->p == '{') {
	           blevel++;
	       } else if (*p->p == '}') {
	           if (blevel != 0) blevel--;
	       }
	       p->p++; p->len--;
	   }
	   p->end = p->p-1;
	   p->type = PT_CMD;
	   if (*p->p == ']') {
	       p->p++; p->len--;
	   }
	*/
}

func (p *picolParser) picolParseVar() int {
	// skip the $
	p.p++
	p.ln--
	p.start = p.p

	for {
		c := p.text[p.p]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			p.p++
			p.ln--
			continue
		}
		break
	}

	if p.start == p.p { // It's just a single char string "$"
		p.start = p.p - 1
		p.end = p.p - 1
		p.type_ = PT_STR
	} else {
		p.end = p.p - 1
		p.type_ = PT_VAR
	}
	return PICOL_OK

	/*
	   p->start = ++p->p; p->len--; // skip the $
	   while(1) {
	       if ((*p->p >= 'a' && *p->p <= 'z') || (*p->p >= 'A' && *p->p <= 'Z') ||
	           (*p->p >= '0' && *p->p <= '9') || *p->p == '_')
	       {
	           p->p++; p->len--; continue;
	       }
	       break;
	   }
	   if (p->start == p->p) { // It's just a single char string "$"
	       p->start = p->end = p->p-1;
	       p->type = PT_STR;
	   } else {
	       p->end = p->p-1;
	       p->type = PT_VAR;
	   }
	*/
}

func (p *picolParser) picolParseBrace() int {
	level := 1
	p.p++
	p.ln--
	p.start = p.p

	for {
		c := p.text[p.p]
		switch {
		case p.ln >= 2 && c == '\\':
			p.p++
			p.ln--
		case p.ln == 0 || c == '}':
			level--
			if level == 0 || p.ln == 0 {
				p.end = p.p - 1
				if p.ln != 0 {
					// Skip final closed brace
					p.p++
					p.ln--

				}
				p.type_ = PT_STR
				return PICOL_OK
			}
		case c == '{':
			level++
		}
		p.p++
		p.ln--
	}
	return PICOL_OK /* unreached */

	/*
	   int level = 1;
	   p->start = ++p->p; p->len--;
	   while(1) {
	       if (p->len >= 2 && *p->p == '\\') {
	           p->p++; p->len--;
	       } else if (p->len == 0 || *p->p == '}') {
	           level--;
	           if (level == 0 || p->len == 0) {
	               p->end = p->p-1;
	               if (p->len) {
	                   p->p++; p->len--; // Skip final closed brace
	               }
	               p->type = PT_STR;
	               return PICOL_OK;
	           }
	       } else if (*p->p == '{')
	           level++;
	       p->p++; p->len--;
	   }
	*/
}

func (p *picolParser) picolParseString() int {
	newword := p.type_ == PT_SEP || p.type_ == PT_EOL || p.type_ == PT_STR
	c := p.text[p.p]
	if newword && c == '{' {
		return p.picolParseBrace()
	} else if newword && c == '"' {
		p.insidequote = 1
		p.p++
		p.ln--
	}

	p.start = p.p

	for {
		if p.ln == 0 {
			p.end = p.p - 1
			p.type_ = PT_ESC
			return PICOL_OK
		}
		switch p.text[p.p] {
		case '\\':
			if p.ln >= 2 {
				p.p++
				p.ln--
			}
		case '$':
		case '[':
			p.end = p.p - 1
			p.type_ = PT_ESC
			return PICOL_OK
		case ' ', '\t', '\n', '\r', ';':
			if p.insidequote == 0 {
				p.end = p.p - 1
				p.type_ = PT_ESC
				return PICOL_OK
			}
		case '"':
			if p.insidequote != 0 {
				p.end = p.p - 1
				p.type_ = PT_ESC
				p.p++
				p.ln--
				p.insidequote = 0
				return PICOL_OK
			}
		}
		p.p++
		p.ln--
	}
	return PICOL_OK /* unreached */

	/*
	   int newword = (p->type == PT_SEP || p->type == PT_EOL || p->type == PT_STR);
	   if (newword && *p->p == '{') return picolParseBrace(p);
	   else if (newword && *p->p == '"') {
	       p->insidequote = 1;
	       p->p++; p->len--;
	   }
	   p->start = p->p;
	   while(1) {
	       if (p->len == 0) {
	           p->end = p->p-1;
	           p->type = PT_ESC;
	           return PICOL_OK;
	       }
	       switch(*p->p) {
	       case '\\':
	           if (p->len >= 2) {
	               p->p++; p->len--;
	           }
	           break;
	       case '$': case '[':
	           p->end = p->p-1;
	           p->type = PT_ESC;
	           return PICOL_OK;
	       case ' ': case '\t': case '\n': case '\r': case ';':
	           if (!p->insidequote) {
	               p->end = p->p-1;
	               p->type = PT_ESC;
	               return PICOL_OK;
	           }
	           break;
	       case '"':
	           if (p->insidequote) {
	              p->end = p->p-1;
	               p->type = PT_ESC;
	               p->p++; p->len--;
	               p->insidequote = 0;
	               return PICOL_OK;
	           }
	           break;
	       }
	       p->p++; p->len--;
	   }
	*/
}

func (p *picolParser) picolParseComment() int {
	for p.ln != 0 && p.text[p.p] != '\n' {
		p.p++
		p.ln--
	}
	return PICOL_OK

	/*
	   while(p->len && *p->p != '\n') {
	       p->p++; p->len--;
	   }
	*/
}

func (p *picolParser) picolGetToken() int {
	for {
		if p.ln == 0 {
			if p.type_ != PT_EOL && p.type_ != PT_EOF {
				p.type_ = PT_EOL
			} else {
				p.type_ = PT_EOF
			}
			return PICOL_OK
		}

		switch p.text[p.p] {
		case ' ', '\t', '\r':
			if p.insidequote != 0 {
				return p.picolParseString()
			}
			return p.picolParseSep()
		case '\n', ';':
			if p.insidequote != 0 {
				return p.picolParseString()
			}
			return p.picolParseEol()
		case '[':
			return p.picolParseCommand()
		case '$':
			return p.picolParseVar()
		case '#':
			if p.type_ == PT_EOL {
				p.picolParseComment()
				continue
			}
			return p.picolParseString()
		default:
			return p.picolParseString()
		}
	}
	return PICOL_OK /* unreached */

	/*
	   while(1) {
	       if (!p->len) {
	           if (p->type != PT_EOL && p->type != PT_EOF)
	               p->type = PT_EOL;
	           else
	               p->type = PT_EOF;
	           return PICOL_OK;
	       }
	       switch(*p->p) {
	       case ' ': case '\t': case '\r':
	           if (p->insidequote) return picolParseString(p);
	           return picolParseSep(p);
	       case '\n': case ';':
	           if (p->insidequote) return picolParseString(p);
	           return picolParseEol(p);
	       case '[':
	           return picolParseCommand(p);
	       case '$':
	           return picolParseVar(p);
	       case '#':
	           if (p->type == PT_EOL) {
	               picolParseComment(p);
	               continue;
	           }
	           return picolParseString(p);
	       default:
	           return picolParseString(p);
	       }
	   }
	*/
	//return PICOL_OK /* unreached */
}

/*




*/

type picolVar struct {
	name, val string
	next      *picolVar

	//char *name, *val;
	//struct picolVar *next;
}

//struct picolInterp; /* forward declaration */
//typedef int (*picolCmdFunc)(struct picolInterp *i, int argc, char **argv, void *privdata);

type picolCmdFunc func(i *picolInterp, argc int, argv []string, privdata interface{}) int

type picolCmd struct {
	name     string
	fn       picolCmdFunc
	privdata interface{}
	next     *picolCmd

	//char *name;
	//picolCmdFunc func;
	//void *privdata;
	//struct picolCmd *next;
}

type picolCallFrame struct {
	vars   *picolVar
	parent *picolCallFrame

	//struct picolVar *vars;
	//struct picolCallFrame *parent; /* parent is NULL at top level */
}

type picolInterp struct {
	level     int
	callframe *picolCallFrame
	commands  *picolCmd
	result    string

	//int level; /* Level of nesting */
	//struct picolCallFrame *callframe;
	//struct picolCmd *commands;
	//char *result;
}

func InitInterp() *picolInterp {
	return &picolInterp{0, &picolCallFrame{}, nil, ""}
	/*
	   i->level = 0;
	   i->callframe = malloc(sizeof(struct picolCallFrame));
	   i->callframe->vars = NULL;
	   i->callframe->parent = NULL;
	   i->commands = NULL;
	   i->result = strdup("");
	*/
}

func (i *picolInterp) SetResult(s string) {
	i.result = s
	/*
	   free(i->result);
	   i->result = strdup(s);
	*/
}

func (i *picolInterp) GetVar(name string) *picolVar {
	for v := i.callframe.vars; v != nil; v = v.next {
		if v.name == name {
			return v
		}
	}
	return nil

	/*
	   struct picolVar *v = i->callframe->vars;
	   while(v) {
	       if (strcmp(v->name,name) == 0) return v;
	       v = v->next;
	   }
	*/
}

func (i *picolInterp) SetVar(name, val string) int {
	v := &picolVar{name, val, i.callframe.vars}
	i.callframe.vars = v
	return PICOL_OK

	/*
	   struct picolVar *v = picolGetVar(i,name);
	   if (v) {
	       free(v->val);
	       v->val = strdup(val);
	   } else {
	       v = malloc(sizeof(*v));
	       v->name = strdup(name);
	       v->val = strdup(val);
	       v->next = i->callframe->vars;
	       i->callframe->vars = v;
	   }
	*/
}

func (i *picolInterp) GetCommand(name string) *picolCmd {
	for c := i.commands; c != nil; c = c.next {
		if c.name == name {
			return c
		}
	}
	return nil

	/*
	   struct picolCmd *c = i->commands;
	   while(c) {
	       if (strcmp(c->name,name) == 0) return c;
	       c = c->next;
	   }
	*/
}

func (i *picolInterp) RegisterCommand(name string, fn picolCmdFunc, privdata interface{}) int {
	c := i.GetCommand(name)
	if c != nil {
		errbuf := fmt.Sprintf("Command '%s' already defined", name)
		i.SetResult(errbuf)
		return PICOL_ERR
	}

	c = &picolCmd{name, fn, privdata, i.commands}
	i.commands = c
	return PICOL_OK

	/*
	   struct picolCmd *c = picolGetCommand(i,name);
	   char errbuf[1024];
	   if (c) {
	       snprintf(errbuf,1024,"Command '%s' already defined",name);
	       picolSetResult(i,errbuf);
	       return PICOL_ERR;
	   }
	   c = malloc(sizeof(*c));
	   c->name = strdup(name);
	   c->func = f;
	   c->privdata = privdata;
	   c->next = i->commands;
	   i->commands = c;
	*/
}

/* EVAL! */
func (i *picolInterp) Eval(t string) int {
	fmt.Printf("::'%s'\n", t)

	p := picolInitParser(t)
	i.SetResult("")

	retcode := PICOL_OK

	argc := 0
	argv := []string{}

	for {
		prevtype := p.type_
		// XXX
		_ = p.picolGetToken()
		if p.type_ == PT_EOF {
			break
		}
		t := p.text[p.start : p.end+1]
		switch p.type_ {
		case PT_VAR:
			v := i.GetVar(t)
			if v == nil {
				errbuf := fmt.Sprintf("No such variable '%s'", t)
				i.SetResult(errbuf)
				retcode = PICOL_ERR
				goto err
			}
			t = v.val
		case PT_CMD:
			retcode = i.Eval(t)
			if retcode != PICOL_OK {
				goto err
			}
			t = i.result
		case PT_ESC:
			// XXX: escape handling missing!
		case PT_SEP:
			prevtype = p.type_
			continue

		}

		// We have a complete command + args. Call it!
		if p.type_ == PT_EOL {
			//struct picolCmd *c;
			//free(t);
			prevtype = p.type_
			if argc != 0 {
				c := i.GetCommand(argv[0])
				if c == nil {
					errbuf := fmt.Sprintf("No such command '%s'", argv[0])
					i.SetResult(errbuf)
					retcode = PICOL_ERR
					goto err
				}
				retcode = c.fn(i, argc, argv, c.privdata)
				if retcode != PICOL_OK {
					goto err
				}
			}
			// Prepare for the next command
			//for (j = 0; j < argc; j++) free(argv[j]);
			//free(argv);
			argv = []string{}
			argc = 0
			continue
		}

		// We have a new token, append to the previous or as new arg?
		if prevtype == PT_SEP || prevtype == PT_EOL {
			argv = append(argv, t)
			argc++
			//argv = realloc(argv, sizeof(char*)*(argc+1));
			//argv[argc] = t;
			//argc++;
		} else { // Interpolation
			argv[argc-1] = strings.Join([]string{argv[argc-1], t}, "")
			/*
			   int oldlen = strlen(argv[argc-1]), tlen = strlen(t);
			   argv[argc-1] = realloc(argv[argc-1], oldlen+tlen+1);
			   memcpy(argv[argc-1]+oldlen, t, tlen);
			   argv[argc-1][oldlen+tlen]='\0';
			   free(t);
			*/
		}
		prevtype = p.type_
	}
err:
	return retcode

	/*
		    struct picolParser p;
		    int argc = 0, j;
		    char **argv = NULL;
		    char errbuf[1024];
		    int retcode = PICOL_OK;
		    picolSetResult(i,"");
		    picolInitParser(&p,t);
		    while(1) {
		        char *t;
		        int tlen;
		        int prevtype = p.type;
		        picolGetToken(&p);
		        if (p.type == PT_EOF) break;
		        tlen = p.end-p.start+1;
		        if (tlen < 0) tlen = 0;
		        t = malloc(tlen+1);
		        memcpy(t, p.start, tlen);
		        t[tlen] = '\0';
		        if (p.type == PT_VAR) {
		            struct picolVar *v = picolGetVar(i,t);
		            if (!v) {
		                snprintf(errbuf,1024,"No such variable '%s'",t);
		                free(t);
		                picolSetResult(i,errbuf);
		                retcode = PICOL_ERR;
		                goto err;
		            }
		            free(t);
		            t = strdup(v->val);
		        } else if (p.type == PT_CMD) {
		            retcode = picolEval(i,t);
		            free(t);
		            if (retcode != PICOL_OK) goto err;
		            t = strdup(i->result);
		        } else if (p.type == PT_ESC) {
		            // XXX: escape handling missing!
		        } else if (p.type == PT_SEP) {
		            prevtype = p.type;
		            free(t);
		            continue;
		        }
		        // We have a complete command + args. Call it!
		        if (p.type == PT_EOL) {
		            struct picolCmd *c;
		            free(t);
		            prevtype = p.type;
		            if (argc) {
		                if ((c = picolGetCommand(i,argv[0])) == NULL) {
		                    snprintf(errbuf,1024,"No such command '%s'",argv[0]);
		                    picolSetResult(i,errbuf);
		                    retcode = PICOL_ERR;
		                    goto err;
		                }
		                retcode = c->func(i,argc,argv,c->privdata);
		                if (retcode != PICOL_OK) goto err;
		            }
		            // Prepare for the next command
		            for (j = 0; j < argc; j++) free(argv[j]);
		            free(argv);
		            argv = NULL;
		            argc = 0;
		            continue;
		        }
		        // We have a new token, append to the previous or as new arg?
		        if (prevtype == PT_SEP || prevtype == PT_EOL) {
		            argv = realloc(argv, sizeof(char*)*(argc+1));
		            argv[argc] = t;
		            argc++;
		        } else { // Interpolation
		            int oldlen = strlen(argv[argc-1]), tlen = strlen(t);
		            argv[argc-1] = realloc(argv[argc-1], oldlen+tlen+1);
		            memcpy(argv[argc-1]+oldlen, t, tlen);
		            argv[argc-1][oldlen+tlen]='\0';
		            free(t);
		        }
		        prevtype = p.type;
		    }
		err:
		    for (j = 0; j < argc; j++) free(argv[j]);
		    free(argv);
		    return retcode;
	*/
	return 0
}

/* ACTUAL COMMANDS! */
func picolArityErr(i *picolInterp, name string, argv []string) int {
	buf := fmt.Sprintf("Wrong number of args for %s", name, argv)
	i.SetResult(buf)
	return PICOL_ERR

	/*
	   char buf[1024];
	   snprintf(buf,1024,"Wrong number of args for %s",name);
	   picolSetResult(i,buf);
	*/
}

func picolCommandMath(i *picolInterp, argc int, argv []string, pd interface{}) int {
	//if argc != 3 {
	if len(argv) != 3 {
		return picolArityErr(i, argv[0], argv)
	}
	//*
	a, _ := strconv.Atoi(argv[1])
	b, _ := strconv.Atoi(argv[2])
	var c int //*/
	/*
		a, _ := strconv.ParseFloat(argv[1], 64)
		b, _ := strconv.ParseFloat(argv[2], 64)
		var c float64 //*/
	switch {
	case argv[0] == "+":
		c = a + b
	case argv[0] == "-":
		c = a - b
	case argv[0] == "*":
		c = a * b
	case argv[0] == "/":
		c = a / b
	case argv[0] == ">":
		if a > b {
			c = 1
		}
	case argv[0] == ">=":
		if a >= b {
			c = 1
		}
	case argv[0] == "<":
		if a < b {
			c = 1
		}
	case argv[0] == "<=":
		if a <= b {
			c = 1
		}
	case argv[0] == "==":
		if a == b {
			c = 1
		}
	case argv[0] == "!=":
		if a != b {
			c = 1
		}
	default: // FIXME I hate warnings
		c = 0
	}
	buf := fmt.Sprintf("%d", c)
	i.SetResult(buf)

	/*
	   char buf[64]; int a, b, c;
	   if (argc != 3) return picolArityErr(i,argv[0]);
	   a = atoi(argv[1]); b = atoi(argv[2]);
	   if (argv[0][0] == '+') c = a+b;
	   else if (argv[0][0] == '-') c = a-b;
	   else if (argv[0][0] == '*') c = a*b;
	   else if (argv[0][0] == '/') c = a/b;
	   else if (argv[0][0] == '>' && argv[0][1] == '\0') c = a > b;
	   else if (argv[0][0] == '>' && argv[0][1] == '=') c = a >= b;
	   else if (argv[0][0] == '<' && argv[0][1] == '\0') c = a < b;
	   else if (argv[0][0] == '<' && argv[0][1] == '=') c = a <= b;
	   else if (argv[0][0] == '=' && argv[0][1] == '=') c = a == b;
	   else if (argv[0][0] == '!' && argv[0][1] == '=') c = a != b;
	   else c = 0; // FIXME I hate warnings
	   snprintf(buf,64,"%d",c);
	   picolSetResult(i,buf);
	*/
	return PICOL_OK
}

func picolCommandSet(i *picolInterp, argc int, argv []string, pd interface{}) int {
	if len(argv) != 3 {
		return picolArityErr(i, argv[0], argv)
	}
	i.SetVar(argv[1], argv[2])
	i.SetResult(argv[2])
	return PICOL_OK

	/*
	   if (argc != 3) return picolArityErr(i,argv[0]);
	   picolSetVar(i,argv[1],argv[2]);
	   picolSetResult(i,argv[2]);
	   return PICOL_OK;
	*/
}

func picolCommandPuts(i *picolInterp, argc int, argv []string, pd interface{}) int {
	if len(argv) != 2 {
		fmt.Println(len(argv), argv[2])
		return picolArityErr(i, argv[0], argv)
	}
	fmt.Println(argv[1])
	return PICOL_OK

	/*
	   if (argc != 2) return picolArityErr(i,argv[0]);
	   printf("%s\n", argv[1]);
	   return PICOL_OK;
	*/
}

func picolCommandIf(i *picolInterp, argc int, argv []string, pd interface{}) int {
	if len(argv) != 3 && len(argv) != 5 {
		return picolArityErr(i, argv[0], argv)
	}
	if retcode := i.Eval(argv[1]); retcode != PICOL_OK {
		return retcode
	}
	if r, _ := strconv.Atoi(i.result); r != 0 {
		return i.Eval(argv[2])
	} else if argc == 5 {
		return i.Eval(argv[5])
	}
	return PICOL_OK

	/*
	   int retcode;
	   if (argc != 3 && argc != 5) return picolArityErr(i,argv[0]);
	   if ((retcode = picolEval(i,argv[1])) != PICOL_OK) return retcode;
	   if (atoi(i->result)) return picolEval(i,argv[2]);
	   else if (argc == 5) return picolEval(i,argv[4]);
	   return PICOL_OK;
	*/
}

func picolCommandWhile(i *picolInterp, argc int, argv []string, pd interface{}) int {
	if len(argv) != 3 {
		return picolArityErr(i, argv[0], argv)
	}
	for {
		retcode := i.Eval(argv[1])
		if retcode != PICOL_OK {
			return retcode
		}
		if r, _ := strconv.Atoi(i.result); r != 0 {
			retcode = i.Eval(argv[2])
			switch retcode {
			case PICOL_CONTINUE, PICOL_OK:
				//pass
			case PICOL_BREAK:
				return PICOL_OK
			default:
				return retcode
			}
		} else {
			return PICOL_OK
		}
	}

	/*
	   if (argc != 3) return picolArityErr(i,argv[0]);
	   while(1) {
	       int retcode = picolEval(i,argv[1]);
	       if (retcode != PICOL_OK) return retcode;
	       if (atoi(i->result)) {
	           if ((retcode = picolEval(i,argv[2])) == PICOL_CONTINUE) continue;
	           else if (retcode == PICOL_OK) continue;
	           else if (retcode == PICOL_BREAK) return PICOL_OK;
	           else return retcode;
	       } else {
	           return PICOL_OK;
	       }
	   }
	*/
}

func picolCommandRetCodes(i *picolInterp, argc int, argv []string, pd interface{}) int {
	if len(argv) != 1 {
		return picolArityErr(i, argv[0], argv)
	}
	switch argv[0] {
	case "break":
		return PICOL_BREAK
	case "continue":
		return PICOL_CONTINUE
	}
	return PICOL_OK

	/*
	   if (argc != 1) return picolArityErr(i,argv[0]);
	   if (strcmp(argv[0],"break") == 0) return PICOL_BREAK;
	   else if (strcmp(argv[0],"continue") == 0) return PICOL_CONTINUE;
	   return PICOL_OK;
	*/
}

func picolDropCallFrame(i *picolInterp) {
	// XXX test it
	i.callframe = i.callframe.parent

	/*
	   struct picolCallFrame *cf = i->callframe;
	   struct picolVar *v = cf->vars, *t;
	   while(v) {
	       t = v->next;
	       free(v->name);
	       free(v->val);
	       free(v);
	       v = t;
	   }
	   i->callframe = cf->parent;
	   free(cf);
	*/
}

func picolCommandCallProc(i *picolInterp, argc int, argv []string, pd interface{}) int {
	fmt.Println(argv, pd)
	var x []string

	if pd, ok := pd.([]string); ok {
		x = pd
		fmt.Println(x)
	} else {
		return PICOL_OK
	}
	return PICOL_OK

	alist := x[0]
	body := x[1]
	p := alist[:]
	arity := 0

	done := false

	errcode := PICOL_OK

	cf := &picolCallFrame{vars: nil, parent: i.callframe}
	i.callframe = cf

	for {
		start := p
		for len(p) != 0 && p[0] != ' ' {
			p = p[1:]
		}
		if len(p) != 0 && p == start {
			p = p[1:]
			continue
		}

		if p == start {
			break
		}
		if len(p) == 0 {
			done = true
		} else {
			p = p[1:1]
		}
		arity++
		if arity > argc-1 {
			goto arityerr
		}
		i.SetVar(start, argv[arity])
		p = p[1:]
		if done {
			break
		}
	}

	//free(tofree);
	if arity != argc-1 {
		goto arityerr
	}
	errcode = i.Eval(body)
	if errcode == PICOL_RETURN {
		errcode = PICOL_OK
	}
	picolDropCallFrame(i) // remove the called proc callframe
	return errcode
arityerr:
	errbuf := fmt.Sprintf("Proc '%s' called with wrong arg num", argv[0])
	//snprintf(errbuf,1024,"Proc '%s' called with wrong arg num",argv[0]);
	i.SetResult(errbuf)
	picolDropCallFrame(i) // remove the called proc callframe
	return PICOL_ERR

	/*
		alist := x[0]
		body := x[1]
		var p, start int

		arity := 0
		done := 0
		errcode := PICOL_OK

		tofree := p

		for {
			start := p
			for alist[p] != ' ' && p != len(alist) {p++}
			if len(alist) != p && p == start {
				p++; continue
			}

			   if p == start{break}
			   if  (*p == '\0') done=1; else *p = '\0';
			   if (++arity > argc-1) goto arityerr;
			   picolSetVar(i,start,argv[arity]);
			   p++;
			   if (done) break;
		}
	*/

	/* TODO
	   char **x=pd, *alist=x[0], *body=x[1], *p=strdup(alist), *tofree;
	   struct picolCallFrame *cf = malloc(sizeof(*cf));
	   int arity = 0, done = 0, errcode = PICOL_OK;
	   char errbuf[1024];
	   cf->vars = NULL;
	   cf->parent = i->callframe;
	   i->callframe = cf;
	   tofree = p;
	   while(1) {
		   char *start = p;
		   while(*p != ' ' && *p != '\0') p++;
		   if (*p != '\0' && p == start) {
			   p++; continue;
		   }
		   if (p == start) break;
		   if (*p == '\0') done=1; else *p = '\0';
		   if (++arity > argc-1) goto arityerr;
		   picolSetVar(i,start,argv[arity]);
		   p++;
		   if (done) break;
	   }
	   free(tofree);
	   if (arity != argc-1) goto arityerr;
	   errcode = picolEval(i,body);
	   if (errcode == PICOL_RETURN) errcode = PICOL_OK;
	   picolDropCallFrame(i); // remove the called proc callframe
	   return errcode;
	arityerr:
	   snprintf(errbuf,1024,"Proc '%s' called with wrong arg num",argv[0]);
	   picolSetResult(i,errbuf);
	   picolDropCallFrame(i); // remove the called proc callframe
	   return PICOL_ERR;
	*/
}

func picolCommandProc(i *picolInterp, argc int, argv []string, pd interface{}) int {
	fmt.Println("proc", argv, pd)

	if len(argv) != 4 {
		return picolArityErr(i, argv[0], argv)
	}
	// FIXME maybe create copy
	procdata := []string{argv[2], argv[3]}
	return i.RegisterCommand(argv[1], picolCommandCallProc, procdata)

	/*
	   char **procdata = malloc(sizeof(char*)*2);
	   if (argc != 4) return picolArityErr(i,argv[0]);
	   procdata[0] = strdup(argv[2]); // arguments list
	   procdata[1] = strdup(argv[3]); // procedure body
	   return picolRegisterCommand(i,argv[1],picolCommandCallProc,procdata);
	*/
}

func picolCommandReturn(i *picolInterp, argc int, argv []string, pd interface{}) int {
	if len(argv) != 1 && len(argv) != 2 {
		return picolArityErr(i, argv[0], argv)
	}
	var r string
	if len(argv) == 2 {
		r = argv[1]
	}
	i.SetResult(r)
	return PICOL_RETURN

	/*
	   if (argc != 1 && argc != 2) return picolArityErr(i,argv[0]);
	   picolSetResult(i, (argc == 2) ? argv[1] : "");
	   return PICOL_RETURN;
	*/
}

func (i *picolInterp) RegisterCoreCommands() {
	name := [...]string{"+", "-", "*", "/", ">", ">=", "<", "<=", "==", "!="}
	for _, n := range name {
		i.RegisterCommand(n, picolCommandMath, nil)
	}
	i.RegisterCommand("set", picolCommandSet, nil)
	i.RegisterCommand("puts", picolCommandPuts, nil)
	i.RegisterCommand("if", picolCommandIf, nil)
	i.RegisterCommand("while", picolCommandWhile, nil)
	i.RegisterCommand("break", picolCommandRetCodes, nil)
	i.RegisterCommand("continue", picolCommandRetCodes, nil)

	i.RegisterCommand("proc", picolCommandProc, nil)
	i.RegisterCommand("return", picolCommandReturn, nil)

	/*
	   int j; char *name[] = {"+","-","*","/",">",">=","<","<=","==","!="};
	   for (j = 0; j < (int)(sizeof(name)/sizeof(char*)); j++)
	       picolRegisterCommand(i,name[j],picolCommandMath,NULL);
	   picolRegisterCommand(i,"set",picolCommandSet,NULL);
	   picolRegisterCommand(i,"puts",picolCommandPuts,NULL);
	   picolRegisterCommand(i,"if",picolCommandIf,NULL);
	   picolRegisterCommand(i,"while",picolCommandWhile,NULL);
	   picolRegisterCommand(i,"break",picolCommandRetCodes,NULL);
	   picolRegisterCommand(i,"continue",picolCommandRetCodes,NULL);
	   picolRegisterCommand(i,"proc",picolCommandProc,NULL);
	   picolRegisterCommand(i,"return",picolCommandReturn,NULL);
	*/
}
