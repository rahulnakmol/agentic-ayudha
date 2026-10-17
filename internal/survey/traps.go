package survey

import (
	"path"
	"regexp"
	"strings"
)

// Traps are places where a legacy stack's semantics differ from what a reader
// (or a model) porting the code would assume: aliasing, implicit typing,
// reflection, monkey patches, default charsets, compiler flags that change
// arithmetic. The index records where each rule matches, one fact per rule and
// file (the first match, with the count), so a claim can cite it; whether the
// trap is real is a question the skill checks in the code, with the per-stack
// questions in skills/survey/references/traps.md (one heading per rule name).
// A rule never decides anything: it says where to look.

// trapRule is one pattern. Langs are language names; Raw rules read the
// source (comments and strings included), the others the stripped code,
// lower-cased for the case-insensitive languages.
type trapRule struct {
	name  string
	langs []string
	raw   bool
	re    *regexp.Regexp
	skip  func(match string) bool // a match that is not the trap
	exts  []string                // only files with these extensions
}

var trapRules = []trapRule{
	// Fortran (lower-cased code).
	{name: "fortran-common", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:\d+[ \t]+)?common\b`)},
	{name: "fortran-equivalence", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*equivalence\b`)},
	{name: "fortran-implicit-rules", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*implicit[ \t]+(?:double|real|integer|logical|complex|character|type)\b`)},
	{name: "fortran-save-data", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:save|data)\b`)},
	{name: "fortran-goto", langs: []string{"fortran"}, re: regexp.MustCompile(`\bgo[ \t]*to\b`)},
	{name: "fortran-arithmetic-if", langs: []string{"fortran"}, re: regexp.MustCompile(`\bif[ \t]*\([^\n]*\)[ \t]*\d+[ \t]*,[ \t]*\d+[ \t]*,[ \t]*\d+`)},
	{name: "fortran-entry", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*entry[ \t]+\w+`)},
	{name: "fortran-assumed-size", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:dimension|real|integer|double[ \t]*precision|complex|logical|character)\b[^\n]*\w[ \t]*\((?:[^()\n]*,)?[ \t]*\*[ \t]*\)`)},
	{name: "fortran-custom-bounds", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:dimension|real|integer|double[ \t]*precision|complex|logical|character|type[ \t]*\()[^\n]*\([ \t]*(?:-[ \t]*\d+|0|[a-z]\w*)[ \t]*:`)},
	{name: "fortran-single-precision", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*real[ \t]*(?:\*[ \t]*4)?(?:[ \t]*,|[ \t]*::|[ \t]+[a-z])`)},
	{name: "fortran-machine-constants", langs: []string{"fortran"}, re: regexp.MustCompile(`\b(?:d1mach|r1mach|i1mach|dpmpar|epsilon|tiny|huge)[ \t]*\(`)},
	{name: "fortran-hollerith", langs: []string{"fortran"}, re: regexp.MustCompile(`[(,][ \t]*\d+h[a-z0-9 ]`)},
	{name: "fortran-format", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:\d+[ \t]+)?format[ \t]*\(`)},
	{name: "fortran-stop", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:\d+[ \t]+)?(?:stop|pause)\b`)},
	{name: "fortran-external", langs: []string{"fortran"}, re: regexp.MustCompile(`(?m)^[ \t]*external\b`)},

	// Ruby.
	{name: "ruby-monkey-patch", langs: []string{"ruby"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:class|module)[ \t]+(?:::)?(?:String|Integer|Fixnum|Bignum|Float|Numeric|Array|Hash|Object|BasicObject|Kernel|Module|Class|Symbol|NilClass|TrueClass|FalseClass|Time|Date|DateTime|Range|Proc|Comparable|Enumerable|IO|File|Dir|Regexp|Struct|Exception|StandardError)[ \t]*(?:$|;)|\brefine[ \t]+[A-Z]\w*`)},
	{name: "ruby-method-missing", langs: []string{"ruby"}, re: regexp.MustCompile(`\bdef[ \t]+(?:self\.)?(?:method_missing|respond_to_missing\?|const_missing)\b`)},
	{name: "ruby-define-method", langs: []string{"ruby"}, re: regexp.MustCompile(`\b(?:define_method|define_singleton_method|instance_variable_set|instance_variable_get|class_variable_set|class_variable_get|const_set|const_get|remove_method|undef_method|alias_method)\b`)},
	{name: "ruby-eval", langs: []string{"ruby"}, re: regexp.MustCompile(`\b(?:instance_eval|class_eval|module_eval|instance_exec|class_exec|eval|binding)\b`)},
	{name: "ruby-send", langs: []string{"ruby"}, re: regexp.MustCompile(`\.(?:send|__send__|public_send)\b`)},
	{name: "ruby-dynamic-require", langs: []string{"ruby"}, re: regexp.MustCompile(`(?m)\b(?:require|require_relative|load)\b[ \t(]+[^'"\s(]|\bautoload\b`)},
	{name: "ruby-globals", langs: []string{"ruby"}, re: regexp.MustCompile(`\$[a-z_]\w*`), skip: func(m string) bool {
		switch m {
		case "$stdout", "$stderr", "$stdin", "$0", "$PROGRAM_NAME", "$LOAD_PATH", "$LOADED_FEATURES", "$DEBUG", "$VERBOSE":
			return true
		}
		return false
	}},
	{name: "ruby-string-mutation", langs: []string{"ruby"}, re: regexp.MustCompile(`\.(?:gsub!|sub!|concat|replace|force_encoding|upcase!|downcase!|capitalize!|strip!|lstrip!|rstrip!|chomp!|chop!|squeeze!|insert|prepend|slice!|tr!)(?:\W|$)|<<[ \t]*["']`)},
	{name: "ruby-symbol-string-keys", langs: []string{"ruby"}, re: regexp.MustCompile(`\.(?:to_sym|intern|stringify_keys|symbolize_keys|with_indifferent_access)\b|HashWithIndifferentAccess`)},
	{name: "ruby-removed-api", langs: []string{"ruby"}, re: regexp.MustCompile(`\b(?:Fixnum|Bignum)\b|\$SAFE|\.(?:taint|untaint|trust|untrust|tainted\?)\b|URI\.(?:escape|encode|decode|unescape)\b|\b(?:File|Dir)\.exists\?`)},
	{name: "ruby-unsafe-load", langs: []string{"ruby"}, re: regexp.MustCompile(`\b(?:Marshal|YAML|Psych)\.load\b`)},
	{name: "ruby-process-hooks", langs: []string{"ruby"}, re: regexp.MustCompile(`\b(?:at_exit|ObjectSpace|trap)\b|Signal\.trap|\bexit!?\b|\babort\b`)},
	{name: "ruby-rails-conventions", langs: []string{"ruby"}, re: regexp.MustCompile(`\bActiveSupport\b|\bRails\.|ActiveRecord::Base|\b(?:mattr_accessor|cattr_accessor|class_attribute|included[ \t]+do|has_many|belongs_to|before_action|before_filter)\b`)},

	// Java.
	{name: "java-reflection", langs: []string{"java", "kotlin", "scala", "groovy"}, re: regexp.MustCompile(`\bClass\.forName[ \t]*\(|\.getDeclared(?:Method|Field|Constructor)s?[ \t]*\(|\.getMethod[ \t]*\(|\.setAccessible[ \t]*\(|\.invoke[ \t]*\(|\.newInstance[ \t]*\(|java\.lang\.reflect\b`)},
	{name: "java-proxy-di", langs: []string{"java", "kotlin"}, re: regexp.MustCompile(`Proxy\.newProxyInstance|@(?:Autowired|Inject|Component|Service|Repository|Controller|RestController|Bean|Configuration|PostConstruct|PreDestroy|Transactional|Aspect|EJB|Stateless|Stateful|Named|Singleton)\b`)},
	{name: "java-classloader", langs: []string{"java", "kotlin", "scala"}, re: regexp.MustCompile(`\b(?:URLClassLoader|ClassLoader)\b|\.defineClass[ \t]*\(|ServiceLoader\.load|getContextClassLoader`)},
	{name: "java-serialization", langs: []string{"java", "kotlin", "scala"}, re: regexp.MustCompile(`\bimplements\b[^{]*\bSerializable\b|\b(?:ObjectInputStream|ObjectOutputStream|Externalizable)\b|\bserialVersionUID\b|\b(?:readObject|writeObject|readResolve|writeReplace)[ \t]*\(`)},
	{name: "java-jni", langs: []string{"java", "kotlin"}, re: regexp.MustCompile(`\bnative[ \t]+[\w<>\[\]]+[ \t]+\w+[ \t]*\(|System\.load(?:Library)?[ \t]*\(`)},
	{name: "java-default-charset", langs: []string{"java", "kotlin", "scala", "groovy"}, re: regexp.MustCompile(`\.getBytes[ \t]*\([ \t]*\)|new[ \t]+(?:InputStreamReader|OutputStreamWriter|FileReader|FileWriter|PrintStream|PrintWriter|Scanner)[ \t]*\([^,()]*(?:\([^()]*\))?[^,()]*\)|Charset\.defaultCharset`)},
	{name: "java-default-locale", langs: []string{"java", "kotlin", "scala", "groovy"}, re: regexp.MustCompile(`\.to(?:Lower|Upper)Case[ \t]*\([ \t]*\)|String\.format[ \t]*\([ \t]*"|Locale\.getDefault|new[ \t]+SimpleDateFormat[ \t]*\([ \t]*"[^"]*"[ \t]*\)|NumberFormat\.get\w*Instance[ \t]*\([ \t]*\)|new[ \t]+DecimalFormat\b`)},
	{name: "java-default-timezone", langs: []string{"java", "kotlin", "scala", "groovy"}, re: regexp.MustCompile(`TimeZone\.(?:getDefault|setDefault)|new[ \t]+Date[ \t]*\([ \t]*\)|Calendar\.getInstance[ \t]*\([ \t]*\)|\b(?:LocalDate|LocalDateTime|LocalTime)\.now[ \t]*\([ \t]*\)|ZoneId\.systemDefault`)},
	{name: "java-threads", langs: []string{"java", "kotlin", "scala"}, re: regexp.MustCompile(`\bsynchronized\b|\bvolatile\b|new[ \t]+Thread[ \t]*\(|\bThreadLocal\b|\bExecutorService\b|\.wait[ \t]*\([ \t]*\)|\.notify(?:All)?[ \t]*\(`)},
	{name: "java-mutable-static", langs: []string{"java"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:(?:public|private|protected)[ \t]+)?static[ \t]+(?:(?:transient|volatile|final)[ \t]+)?[\w<>\[\], .?]+[ \t]+\w+[ \t]*(?:=|;)`), skip: func(m string) bool {
		return regexp.MustCompile(`\bfinal\b`).MatchString(m)
	}},
	{name: "java-swallowed-exception", langs: []string{"java", "kotlin", "groovy"}, re: regexp.MustCompile(`catch[ \t]*\([^)]*\)[ \t\n]*\{[ \t\n]*\}`)},
	{name: "java-internal-api", langs: []string{"java", "kotlin", "scala"}, re: regexp.MustCompile(`\bsun\.misc\b|\bcom\.sun\.\w|\bjdk\.internal\b|\bUnsafe\b`)},
	{name: "java-exit", langs: []string{"java", "kotlin", "scala", "groovy"}, re: regexp.MustCompile(`System\.exit[ \t]*\(|Runtime\.getRuntime\(\)\.(?:halt|addShutdownHook)|void[ \t]+finalize[ \t]*\(`)},

	// C and C++.
	{name: "c-unbounded-copy", langs: []string{"c", "cpp", "objc"}, re: regexp.MustCompile(`\b(?:strcpy|strcat|sprintf|vsprintf|gets|scanf|sscanf)[ \t]*\(`)},
	{name: "c-global-state", langs: []string{"c", "cpp"}, re: regexp.MustCompile(`(?m)^(?:static[ \t]+)?(?:unsigned[ \t]+|signed[ \t]+|long[ \t]+|short[ \t]+)*(?:int|char|double|float|bool|long|size_t|[A-Z]\w*)[ \t]*\*?[ \t]*\w+(?:[ \t]*\[[^\]]*\])?[ \t]*(?:=[^;(]*)?;`), skip: func(m string) bool {
		return strings.HasPrefix(m, "return") || strings.Contains(m, "const") || strings.HasPrefix(m, "typedef")
	}},
	{name: "c-setjmp-signal", langs: []string{"c", "cpp"}, re: regexp.MustCompile(`\b(?:setjmp|longjmp|sigsetjmp|siglongjmp|signal|sigaction)[ \t]*\(`)},
	{name: "c-locale", langs: []string{"c", "cpp"}, re: regexp.MustCompile(`\b(?:setlocale|localeconv|strtod|atof|printf)[ \t]*\(`)},
	{name: "c-type-punning", langs: []string{"c", "cpp"}, re: regexp.MustCompile(`\bunion\b|reinterpret_cast|\([ \t]*(?:unsigned[ \t]+)?(?:char|void|int|long)[ \t]*\*[ \t]*\)[ \t]*&`)},
	{name: "c-layout-endianness", langs: []string{"c", "cpp"}, raw: true, re: regexp.MustCompile(`#[ \t]*pragma[ \t]+pack|__attribute__[ \t]*\(\([ \t]*packed|\b(?:htonl|htons|ntohl|ntohs|__BYTE_ORDER__|BIG_ENDIAN|LITTLE_ENDIAN)\b`)},

	// PHP.
	{name: "php-dynamic-names", langs: []string{"php"}, re: regexp.MustCompile(`\$\$\w+|\$\{[ \t]*\$|\b(?:extract|compact)[ \t]*\(`)},
	{name: "php-eval", langs: []string{"php"}, re: regexp.MustCompile(`\b(?:eval|create_function|assert)[ \t]*\(|\bpreg_replace[ \t]*\([ \t]*'[^']*/e'`)},
	{name: "php-error-suppression", langs: []string{"php"}, re: regexp.MustCompile(`@[ \t]*\$\w|@[ \t]*\w+[ \t]*\(`)},
	{name: "php-removed-api", langs: []string{"php"}, re: regexp.MustCompile(`\b(?:mysql_\w+|ereg\w*|split|each|mcrypt_\w+|money_format|get_magic_quotes_\w+)[ \t]*\(`)},
	{name: "php-globals", langs: []string{"php"}, re: regexp.MustCompile(`\bglobal[ \t]+\$|\$GLOBALS\b`)},
	{name: "php-magic-methods", langs: []string{"php"}, re: regexp.MustCompile(`function[ \t]+__(?:get|set|call|callStatic|isset|unset|invoke)\b`)},

	// Python.
	{name: "python-dynamic", langs: []string{"python"}, re: regexp.MustCompile(`\b(?:exec|eval|__import__|setattr|delattr)[ \t]*\(|importlib\.import_module|\b__getattr__\b`)},
	{name: "python-mutable-default", langs: []string{"python"}, re: regexp.MustCompile(`def[ \t]+\w+[ \t]*\([^)]*=[ \t]*(?:\[[ \t]*\]|\{[ \t]*\}|set\([ \t]*\))`)},
	{name: "python-global", langs: []string{"python"}, re: regexp.MustCompile(`(?m)^[ \t]+(?:global|nonlocal)[ \t]+\w+`)},
	{name: "python-2-only", langs: []string{"python"}, re: regexp.MustCompile(`(?m)^[ \t]*print[ \t]+[^(=\s]|\b(?:xrange|unicode|raw_input|execfile|reduce)[ \t]*\(|\.(?:has_key|iteritems|itervalues|iterkeys)[ \t]*\(|except[ \t]+\w+[ \t]*,[ \t]*\w+[ \t]*:`)},
	{name: "python-unsafe-load", langs: []string{"python"}, re: regexp.MustCompile(`\b(?:pickle|cPickle|marshal|shelve)\.loads?\b|\byaml\.load[ \t]*\(`)},
	{name: "python-star-import", langs: []string{"python"}, re: regexp.MustCompile(`(?m)^[ \t]*from[ \t]+[\w.]+[ \t]+import[ \t]+\*`)},

	// JavaScript and TypeScript.
	{name: "js-prototype-patch", langs: []string{"javascript", "typescript"}, re: regexp.MustCompile(`\b(?:Array|Object|String|Function|Number|Date|RegExp|Promise|Element|HTMLElement|Node)\.prototype\.\w+[ \t]*=`)},
	{name: "js-eval", langs: []string{"javascript", "typescript"}, re: regexp.MustCompile(`\beval[ \t]*\(|new[ \t]+Function[ \t]*\(|\bwith[ \t]*\(|set(?:Timeout|Interval)[ \t]*\([ \t]*["']`)},
	{name: "js-globals", langs: []string{"javascript", "typescript"}, re: regexp.MustCompile(`\b(?:window|global|globalThis|self)\.\w+[ \t]*=[^=]`)},
	{name: "js-loose-equality", langs: []string{"javascript"}, re: regexp.MustCompile(`[^=!<>]==[^=]|!=[^=]`)},
	{name: "js-parseint-radix", langs: []string{"javascript", "typescript"}, re: regexp.MustCompile(`\bparseInt[ \t]*\([^,()]+\)`)},

	// COBOL (lower-cased code).
	{name: "cobol-redefines", langs: []string{"cobol"}, re: regexp.MustCompile(`\bredefines\b`)},
	{name: "cobol-binary-decimal", langs: []string{"cobol"}, re: regexp.MustCompile(`\b(?:comp-3|packed-decimal|comp-[1245]|comp|computational(?:-\d)?|binary)\b`)},
	{name: "cobol-alter", langs: []string{"cobol"}, re: regexp.MustCompile(`\balter\b`)},
	{name: "cobol-goto", langs: []string{"cobol"}, re: regexp.MustCompile(`\bgo[ \t]+to\b`)},
	{name: "cobol-occurs-depending", langs: []string{"cobol"}, re: regexp.MustCompile(`\boccurs\b[^.]*\bdepending[ \t]+on\b`)},
	{name: "cobol-copy-replacing", langs: []string{"cobol"}, re: regexp.MustCompile(`\bcopy\b[^.]*\breplacing\b`)},
	{name: "cobol-collating", langs: []string{"cobol"}, re: regexp.MustCompile(`\bcollating[ \t]+sequence\b|\bebcdic\b|\bcode-set\b`)},
	{name: "cobol-signed-picture", langs: []string{"cobol"}, re: regexp.MustCompile(`\bpic(?:ture)?[ \t]+(?:is[ \t]+)?s9|\bsign[ \t]+(?:is[ \t]+)?(?:leading|trailing)\b`)},
	{name: "cobol-edited-picture", langs: []string{"cobol"}, re: regexp.MustCompile(`\bpic(?:ture)?[ \t]+(?:is[ \t]+)?[^\s.]*[zb$,*+-][^\s.]*`)},
	{name: "cobol-dynamic-call", langs: []string{"cobol"}, re: regexp.MustCompile(`\bcall[ \t]+[a-z]`)},
	{name: "cobol-indexed-file", langs: []string{"cobol"}, re: regexp.MustCompile(`\borganization[ \t]+(?:is[ \t]+)?(?:indexed|relative)\b`)},

	// Visual Basic (lower-cased code). VB6 and VBA only where the 16-bit
	// Integer and the default property rules apply.
	{name: "vb-on-error", langs: []string{"vb"}, re: regexp.MustCompile(`\bon[ \t]+error[ \t]+(?:resume[ \t]+next|goto)\b`)},
	{name: "vb-option-base-compare", langs: []string{"vb"}, re: regexp.MustCompile(`\boption[ \t]+(?:base[ \t]+1|compare[ \t]+text)\b`)},
	{name: "vb-16bit-integer", langs: []string{"vb"}, exts: []string{".bas", ".cls", ".frm", ".ctl", ".vba"}, re: regexp.MustCompile(`\bas[ \t]+integer\b|\bcint[ \t]*\(`)},
	{name: "vb-variant-currency", langs: []string{"vb"}, re: regexp.MustCompile(`\bas[ \t]+(?:variant|currency|object)\b|\bccur[ \t]*\(`)},
	{name: "vb-com-late-binding", langs: []string{"vb"}, re: regexp.MustCompile(`\b(?:createobject|getobject)[ \t]*\(`)},
	{name: "vb-win32-declare", langs: []string{"vb"}, re: regexp.MustCompile(`(?m)^[ \t]*(?:(?:public|private)[ \t]+)?declare[ \t]+(?:ptrsafe[ \t]+)?(?:function|sub)\b`)},
	{name: "vb-doevents-sendkeys", langs: []string{"vb"}, re: regexp.MustCompile(`\b(?:doevents|sendkeys)\b`)},
	{name: "vb-registry", langs: []string{"vb"}, re: regexp.MustCompile(`\b(?:getsetting|savesetting|deletesetting|getallsettings|reg(?:open|create|query|set|delete|close)\w*)\b`)},
	{name: "vb-locale-conversion", langs: []string{"vb"}, re: regexp.MustCompile(`\b(?:val|cdate|datevalue|timevalue|format\$?|cstr|cdbl|csng|isdate|isnumeric)[ \t]*\(`)},
	{name: "vb-data-access", langs: []string{"vb"}, raw: true, re: regexp.MustCompile(`(?i)\b(?:OpenDatabase|DBEngine|CreateDatabase|OpenRecordset|ADODB\.\w+|DAO\.\w+)\b|\.mdb\b|\.accdb\b|Microsoft\.(?:Jet|ACE)\.OLEDB`)},
	{name: "vb-file-io", langs: []string{"vb"}, re: regexp.MustCompile(`\bopen\b[^\n]*\bfor[ \t]+(?:input|output|append|binary|random)\b|\b(?:get|put|print|write|line[ \t]+input)[ \t]*#`)},
	{name: "vb-fixed-string", langs: []string{"vb"}, re: regexp.MustCompile(`\bas[ \t]+string[ \t]*\*[ \t]*\d+`)},
	{name: "vb-desktop-integration", langs: []string{"vb"}, re: regexp.MustCompile(`\b(?:printer|clipboard|app|screen)\.\w+|\b(?:printform|shell|chdir|chdrive|curdir)\b`)},

	// Pascal and Delphi (lower-cased code; directives from the source).
	{name: "pascal-absolute", langs: []string{"pascal"}, re: regexp.MustCompile(`\babsolute\b`)},
	{name: "pascal-with", langs: []string{"pascal"}, re: regexp.MustCompile(`\bwith\b[^;]*\bdo\b`)},
	{name: "pascal-directives", langs: []string{"pascal"}, raw: true, re: regexp.MustCompile(`(?i)\{\$(?:[RQIHJ][+-]|RANGECHECKS|OVERFLOWCHECKS|IOCHECKS|LONGSTRINGS|WRITEABLECONST|A[1248+-]|ALIGN|MODE[ \t]+\w+|H-)`)},
	{name: "pascal-raw-memory", langs: []string{"pascal"}, re: regexp.MustCompile(`\b(?:getmem|freemem|reallocmem|move|fillchar)[ \t]*\(|\^[a-z]`)},
	{name: "pascal-string-types", langs: []string{"pascal"}, re: regexp.MustCompile(`\bshortstring\b|\bstring[ \t]*\[[ \t]*\d+[ \t]*\]|\bansistring\b|\bpchar\b`)},
	{name: "pascal-inline-asm", langs: []string{"pascal"}, re: regexp.MustCompile(`\basm\b`)},
}

// generatedRule marks a generated file: porting generated code by hand is a
// trap; port (or replace) the generator's input instead. Read from the first
// 30 lines of any source file.
var generatedRule = regexp.MustCompile(`(?i)\b(?:generated[ \t]+by|do[ \t]+not[ \t]+edit|auto-?generated|@generated|this[ \t]+file[ \t]+was[ \t]+generated)\b`)

// buildFlagRule finds compiler and runtime flags in build files that change
// a program's semantics: arithmetic, default kinds, static locals, the
// language level, encodings and time zones.
var buildFlagRule = regexp.MustCompile(`(?:^|[\s"'=(,])(-ffast-math|-Ofast|-march=native|-xHOST|-no-prec-div|-ffp-contract=fast|-funsafe-math-optimizations|-ffloat-store|-mfpmath=387|-fno-strict-aliasing|-fwrapv|-fpermissive|-funsigned-char|-fdefault-real-8|-fdefault-double-8|-fdefault-integer-8|-freal-4-real-8|-fno-automatic|-finit-local-zero|-finit-real=\w+|-std=legacy|-ff2c|-fno-range-check|-fcray-pointer|-fdec\S*|-fbackslash|-r8|-i8|-m32|/fp:fast|-fp-model[= ]fast\S*|-Dfile\.encoding=\S+|-Duser\.timezone=\S+|-Duser\.language=\S+|-source[ \t]+1\.[1-7]|-target[ \t]+1\.[1-7])`)

// buildComment is a # or // comment in a build file: at a line's start or
// after a space, so a URL or "a#b" is left alone.
var buildComment = regexp.MustCompile(`(?m)(?:^|[ \t])(?:#|//)[^\n]*`)

// traps records the trap facts of one source file.
func (ix *Index) traps(file, lang, code, raw string) {
	ext := strings.ToLower(path.Ext(file))
	lower := code
	switch lang {
	case "fortran", "cobol", "vb", "pascal":
		lower = strings.ToLower(code)
	}
	add := func(rule string, text string, locs [][]int, detail string) {
		if len(locs) == 0 {
			return
		}
		line := strings.Count(text[:locs[0][0]], "\n") + 1
		if detail == "" {
			detail = strings.TrimSpace(text[locs[0][0]:locs[0][1]])
		}
		ix.Traps = append(ix.Traps, Fact{ID: "trap:" + rule + ":" + file + ":" + itoa(line), Kind: rule, At: file + ":" + itoa(line), Detail: clip(detail, 120), Count: len(locs)})
	}
	for _, r := range trapRules {
		if !contains(r.langs, lang) || len(r.exts) > 0 && !contains(r.exts, ext) {
			continue
		}
		text := lower
		if r.raw {
			text = raw
		}
		locs := r.re.FindAllStringIndex(text, -1)
		if r.skip != nil {
			kept := locs[:0:0]
			for _, l := range locs {
				if !r.skip(text[l[0]:l[1]]) {
					kept = append(kept, l)
				}
			}
			locs = kept
		}
		add(r.name, text, locs, "")
	}
	head := raw
	if lines := strings.SplitN(raw, "\n", 31); len(lines) > 30 {
		head = strings.Join(lines[:30], "\n")
	}
	add("generated-code", head, generatedRule.FindAllStringIndex(head, 1), "")
	if lang == "vb" && ext != ".vb" && ext != ".vbs" && strings.TrimSpace(code) != "" && !strings.Contains(lower, "option explicit") {
		ix.Traps = append(ix.Traps, Fact{ID: "trap:vb-no-option-explicit:" + file + ":1", Kind: "vb-no-option-explicit", At: file + ":1",
			Detail: "no Option Explicit: an undeclared name is a new Variant", Count: 1})
	}
}

// buildTraps records the semantics-changing flags of one build file. A
// commented-out flag is not in the build: # and // comments are blanked
// first, keeping quoted strings, where build files put their flags.
func (ix *Index) buildTraps(file, raw string) {
	raw = buildComment.ReplaceAllStringFunc(raw, func(c string) string { return strings.Repeat(" ", len(c)) })
	locs := buildFlagRule.FindAllStringSubmatchIndex(raw, -1)
	if len(locs) == 0 {
		return
	}
	seen := map[string]bool{}
	var flags []string
	for _, l := range locs {
		if f := raw[l[2]:l[3]]; !seen[f] {
			seen[f] = true
			flags = append(flags, f)
		}
	}
	line := strings.Count(raw[:locs[0][2]], "\n") + 1
	ix.Traps = append(ix.Traps, Fact{ID: "trap:build-semantic-flags:" + file + ":" + itoa(line), Kind: "build-semantic-flags", At: file + ":" + itoa(line),
		Detail: clip(strings.Join(flags, " "), 120), Count: len(locs)})
}

// TrapRules lists every trap rule name, for the skill's reference check.
func TrapRules() []string {
	out := []string{"build-semantic-flags", "fortran-implicit-typing", "generated-code", "vb-no-option-explicit"}
	for _, r := range trapRules {
		out = append(out, r.name)
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
