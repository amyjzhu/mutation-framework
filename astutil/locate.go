package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"math/rand"
	"strconv"
	"bitbucket.org/bestchai/dinv/capture"
	"bitbucket.org/bestchai/dinv/programslicer"
	"errors"
	"fmt"
)

func CreateReadZeroAssignment(node ast.Node, info *types.Info) *ast.AssignStmt {
	if assign, ok := node.(*ast.AssignStmt); ok {
		lhs := assign.Lhs
		rhs := assign.Rhs
		for _, expr := range rhs {
			if isReadOperation(expr) {
				return createZeroAssignment(lhs, assign.End(), info)
			}
		}
	}
	return nil
}


func isReadOperation(node ast.Node) bool {
	// check if it's read
	if callExpr, ok := node.(*ast.CallExpr); ok {
		callFun := callExpr.Fun
		if selectorExpr, ok := callFun.(*ast.SelectorExpr); ok {
			connectionObject := selectorExpr.X
			connectionFn := selectorExpr.Sel
			if strings.EqualFold(connectionFn.Name, "read") {
				// TODO check if X type is Conn
				_ = connectionObject
				return true
			}
		}
	}

	return false
}

// TODO handle nil
func createZeroAssignment(assigns []ast.Expr, endPos token.Pos, info *types.Info) *ast.AssignStmt {
	for _, assign := range assigns {
		if identifier, ok := assign.(*ast.Ident); ok {

			possibleReadBytesVar := findReadBytesVariable(identifier, info)
			if possibleReadBytesVar != nil {
				if possibleReadBytesVar.Type().String() != "error" {

					return constructZeroAssignment(identifier, endPos+1)
				}
			}
		}
	}

	return nil
}

func findReadBytesVariable(identifier *ast.Ident, info *types.Info) types.Object {
	possibleReadBytesVar := info.Uses[identifier]
	if possibleReadBytesVar == nil {
		possibleReadBytesVar = info.Defs[identifier]
	}

	return possibleReadBytesVar
}

func constructZeroAssignment(identifier *ast.Ident, endPos token.Pos) *ast.AssignStmt {
	assignLhs := []ast.Expr{identifier}
	assignRhs := []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}}
	return &ast.AssignStmt{Rhs: assignRhs, Lhs: assignLhs, TokPos: endPos + 1, Tok:token.ASSIGN}
}

func SwapProtocolVersion(node ast.Node, info *types.Info) (func(), func()) {
	// if it's a udp4, udp6, etc. switch for another type
	// TODO handle nil objects
	if call, ok := node.(*ast.CallExpr); ok {
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
			originalArg := call.Args[0]
			switch strings.ToLower(selector.Sel.Name) {
			case "listentcp":
				return func() { swapTcpVersion(call) },
				func() { call.Args[0] = originalArg }
			case "listenudp":
				return func() { swapUdpVersion(call) },
					func() { call.Args[0] = originalArg }
			default:
				// do nothing
			}
			//if strings.EqualFold(selector.Sel.Name, "listentcp") {}
		}
	}

	return nil, nil
}

type NetworkVersion int

const (
	PLAIN NetworkVersion = iota
	FOUR
	SIX
)

var NetworkVersionMap = map[NetworkVersion]string{
	PLAIN: "",
	FOUR: "4",
	SIX: "6",
}

func (version NetworkVersion) getStringRepresentation() string {
	return NetworkVersionMap[version]
}

func (version NetworkVersion) getOtherVersion() NetworkVersion {
	randomQuantity := rand.Int()
	return NetworkVersion((int(version) + randomQuantity) % 3)
}

func swapTcpVersion(call *ast.CallExpr) {
	swapNetworkVersion("tcp", call)
}

func swapUdpVersion(call *ast.CallExpr) {
	swapNetworkVersion("udp", call)
}

func swapNetworkVersion(protocol string, call *ast.CallExpr) {
	arguments := call.Args
	networkArgument := arguments[0]
	if arg, ok := networkArgument.(*ast.Ident); ok {
		networkVersion, err := getNetworkVersion(arg.Name)
		if err != nil {
			return
		}
		newVersion := networkVersion.getOtherVersion()
		newString := protocol + newVersion.getStringRepresentation()
		newIdent := ast.NewIdent(newString)
		call.Args[0] = newIdent
	}
}

func getNetworkVersion(versionString string) (NetworkVersion, error) {
	versionRegex := `^[\a-zA-Z]*([\d]?)$`
	versionMatcher := regexp.MustCompile(versionRegex)
	matches := versionMatcher.FindAllStringSubmatch(versionString, -1)
	matchIndex := 0
	capturingGroup := 1
	numericVersion := matches[matchIndex][capturingGroup]
	int, err := strconv.Atoi(numericVersion)
	return NetworkVersion(int), err
}

func SwapUnderlyingProtocol() {

}


func IsErrorHandlingCode(n ast.Node, info *types.Info) (bool, *ast.BlockStmt) {
	ret, ok := n.(*ast.IfStmt)
	if ok {
		testCond, ok := ret.Cond.(*ast.BinaryExpr)
		// does not cover every error-handling case, but covers most common
		// i.e. err != nil or err == nil
		// another tactic could be dataflow following where errors are raised (TODO)
		if ok {
			if isConditionalErrorHandling(testCond, info) {

				// err != nil
				if testCond.Op == token.NEQ {
					// return the body
					return true, ret.Body

				// err == nil
				} else if testCond.Op == token.EQL {
					if ret.Else != nil {
						ifst, ok := ret.Else.(*ast.IfStmt)
						if ok {
							// i.e. err == nil {} else if { ... }
							return IsErrorHandlingCode(ifst, info)
						} else {
							// the else block actually has error handling
							// i.e. err == nil {} else { ... }
							if block, ok := ret.Else.(*ast.BlockStmt); ok {
								return true, block
							}
						}
					}
				}
			}
		}
	}

	return false, nil
}

func isConditionalErrorHandling(testCond *ast.BinaryExpr, info *types.Info) bool {
	xId, ok := testCond.X.(*ast.Ident)
	if !ok {
		return false
	}
	yId, ok := testCond.Y.(*ast.Ident)
	if !ok {
		return false
	}

	xObj := info.Uses[xId]
	yObj := info.Uses[yId]

	if xObj != nil && yObj != nil {
		if xObj.Type().String() == "error" &&
			isUntypedNil(yObj) ||
			yObj.Type().String() == "error" &&
				isUntypedNil(xObj) {
			return true
		}
	}
	return false
}

func isUntypedNil(candidate types.Object) bool {
	// TODO can candidate.Type() return nil?
	basicType, ok := candidate.Type().(*types.Basic)
	if ok {
		if basicType.Kind() == types.UntypedNil {
			return true
		}
	}
	return false
}

func getErrorBlockNodes(node *ast.File, info *types.Info) []*ast.BlockStmt {

	var bodies []*ast.BlockStmt

	ast.Inspect(node, func(n ast.Node) bool {
		// TODO necessary?
		if n == nil {
			return false
		}

		errorHandling, block := IsErrorHandlingCode(n, info)
		if errorHandling && block != nil {
			bodies = append(bodies, block)
			return true
		}

		return true
	})

	return bodies
}

// check if a call is a time.Sleep
func IsTimeoutCall(call *ast.CallExpr, info *types.Info) bool {
	// don't waste time computing if selector doesn't match
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := fun.Sel.Name
		if !strings.EqualFold(funcName, "Sleep") {
			return false
		} else {
			return selectorMatchesTimeLibrary(fun, info)
		}
	} else {
		// not a selector expression
		return false
	}

}

func selectorMatchesTimeLibrary(fun *ast.SelectorExpr, info *types.Info) bool {
	const TimePackagePath = "time"
	timeLibraryName := "time"

	// implicits is where you find non-renamed package imports
	for ident, obj := range info.Implicits {
		// if there are any imports here
		if dependency, ok := ident.(*ast.ImportSpec); ok {
			// check that they correspond to path of time package "time"
			objName := obj.Name()
			if dependency.Path.Value == TimePackagePath {
				timeLibraryName = objName
			}
		}
	}

	// otherwise, maybe package was renamed
	for identifiers, obj := range info.Defs {
		if obj != nil {
			if isTimeLibrary(obj.String()) {
				// ideally interchangeable
				timeLibraryName = identifiers.Name
				// timeLibraryName = obj.Name()
			}
		}
	}

	// find the libraryName of the library
	// assuming it's an ident and not another expression
	libraryName, ok := fun.X.(*ast.Ident)
	if ok {
		funcLibrary := libraryName.Name

		// then we compare the library and function call
		if funcLibrary == timeLibraryName {
			return true
		}
	}
	return false
}

// object string apparently resembles "package alias ("time")"
// where alias is user-defined variable
func isTimeLibrary(testString string) bool {
	const timeLibraryObjString = `package ([\w]*) \("time"\)`
	timeLibraryRegex := regexp.MustCompile(timeLibraryObjString)

	return timeLibraryRegex.MatchString(testString)
}







// TODO ================================



func getLoggingDenseAreas() {

}

// TODO perhaps specified by test administrator, since
// this could be difficult to automatically find
func getServiceStartupCode() {

}

func getServiceShutdownCode() {

}


// =====================================
// don't need tcp/udp library?



const (
	SEND = iota
	RECEIVE
	BOTH
	NOT
)


func IsSendingMessageNode() error {
	// get the library of network calls in a package
	// maybe just do this once at initialization... don't want to continually load
	if database != nil || len(database) == 0 {
		return errors.New("network database is not loaded or there are no network calls")
	}

	return nil
}

func getCallExpressionFromNode(node ast.Node) *ast.CallExpr {
	var callExpr *ast.CallExpr
	var ok bool
	switch commExpr := node.(type) {
	case *ast.ExprStmt:
		callExpr, ok = commExpr.X.(*ast.CallExpr)
		if ok {
			break
		}
	case *ast.AssignStmt:
		commAss, ok := node.(*ast.AssignStmt)
		if ok {
			// TOOD cause dereference error/NPR?
			callExpr, ok = commAss.Rhs[0].(*ast.CallExpr)
			if ok {
				break
			}
		}
	}

	return callExpr
}


// TODO not sure how to finish this
func replaceCommunicationNode(node ast.Node, info *types.Info)  {
	// need the types.Object to find the actual type unfortunately
	// need to be ast.Ident?
	// then check ident name against object name, find package of
	// ident originally, and then compare to netconn?

	// actually should check and give what kind of node it is
	// and whether send/receive/both

	callExpr := getCallExpressionFromNode(node)

	if callExpr == nil {
		return
	}
	// get package
	// get object
	sel, ok := callExpr.Fun.(*ast.SelectorExpr)
	if ok {
		libIdent, ok := sel.X.(*ast.Ident)
		if ok {
			// check ident against info
			// where are packages declared?
			// i.e. what if aliased
			// TODO this is not complete
			libObject := getImportObject(libIdent, info)
			netConn := database[libObject]

			// get the object of the function call
			selIdent := sel.Sel
			allFunctions := append(netConn.SenderFunctions, append(netConn.ConnectionFunctions, netConn.ReceivingFunctions...)...)
			for _, netFunc := range allFunctions {
				if netFunc.Name == selIdent.Name {

				}
			}

			// choose another call with same amount of arguments
			// and from same range
			newCall, newLib, err := findSubstitutableNetworkCall(&libObject, selIdent)
			if err != nil {
				// TODO breaks if argument list differs in order
				// TODO add imports
				// TODO make actual variables...? how can I replace function w/o
				// replacing connection object?
				newLibIdent := ast.NewIdent(newLib.NetType)
				newSelectIdent := ast.NewIdent(newCall.Name)
				newSelectorExpr := &ast.SelectorExpr{X:newLibIdent, Sel:newSelectIdent}
				callExpr.Fun = newSelectorExpr
			}

			fmt.Print(netConn)
		}

	}


}

func findSubstitutableNetworkCall(lib *types.Object, sel *ast.Ident) (*capture.NetFunc, *capture.NetConn, error) {
	// should be from same category send/receive, same arguments
	if database == nil {
		return nil, nil, errors.New("database not initialized")
	}

	originalNetConn := database[*lib]
	originalType := getCallType(sel, *originalNetConn)

	var replacementFunc *capture.NetFunc
	var replacementLib *capture.NetConn
	var err error
	//random := rand.Int() % (len(database) - 1)
	for name, netConn := range database {
		if *lib == name {
			continue
		}
		replacementLib = netConn

		var originalCall *capture.NetFunc
		switch originalType {
		case SEND:
			originalCall = getOriginalCall(sel, originalNetConn.SenderFunctions)
			replacementFunc, err = getReplacementCall(originalCall, replacementLib.SenderFunctions)
		case RECEIVE:
			originalCall = getOriginalCall(sel, originalNetConn.ReceivingFunctions)
			replacementFunc, err = getReplacementCall(originalCall, replacementLib.ReceivingFunctions)
		case BOTH:
			originalCall = getOriginalCall(sel, originalNetConn.ConnectionFunctions)
			replacementFunc, err = getReplacementCall(originalCall, replacementLib.ConnectionFunctions)
		default:
		}
	}

	if err != nil {
		return replacementFunc, replacementLib, nil
	} else {
		return nil, nil, err
	}
}

func getReplacementCall(original *capture.NetFunc, candidates []*capture.NetFunc) (*capture.NetFunc, error) {
	numArgs := original.Args
	returnVals := original.Returns

	// TODO make sure these are loose/deep equals
	for _, netFunc := range candidates {
		if numArgs == netFunc.Args &&
			returnVals == netFunc.Returns {
			return netFunc, nil
		}
	}

	return nil, errors.New("could not find suitable replacement call")
	// else impossible
	// TODO return error == skip this mutation
}

func getOriginalCall(sel *ast.Ident, funcs []*capture.NetFunc) *capture.NetFunc {
	for _, netFunc := range funcs {
		if netFunc.Name == sel.Name {
			return netFunc
		}
	}

	return nil
}

func getCallType(sel *ast.Ident, conn capture.NetConn) int {
	if contains(sel.Name, conn.SenderFunctions) {
		return SEND
	} else if contains(sel.Name, conn.ReceivingFunctions) {
		return RECEIVE
	} else if contains(sel.Name, conn.ConnectionFunctions) {
		return BOTH
	} else {
		return NOT
	}
}

func contains(name string, funcs []*capture.NetFunc) bool {
	for _, netFunc := range funcs {
		if netFunc.Name == name {
			return true
		}
	}
	return false
}

func getImportObject(ident *ast.Ident, info *types.Info) types.Object {
	for node, obj := range info.Implicits {
		if importSpec, ok := node.(*ast.ImportSpec); ok {
			if importSpec.Name == ident {
				return obj
			}
		}
	}

	// Check defs
	importSpec := info.Defs[ident]
	if importSpec != nil {
		return importSpec
	}

	// Check uses
	importSpec = info.Uses[ident]
	if importSpec != nil {
		return importSpec
	}

	return nil
}

var database map[types.Object]*capture.NetConn
// call once per mutation file
func initializeNetworkDb(path string) error {
	wrapper, err := programslicer.GetProgramWrapperFile(path)
	if err != nil {
		return err
	}

	database = capture.GetNetConns(wrapper)

	return nil
}

// ====================