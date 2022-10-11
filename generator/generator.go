package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/exp/slices"

	"github.com/Bananenpro/embe/analyzer"
	"github.com/Bananenpro/embe/blocks"
	"github.com/Bananenpro/embe/parser"
)

func GenerateBlocks(statements []parser.Stmt, definitions analyzer.Definitions, lines [][]rune) (map[string]*blocks.Block, []error) {
	blocks.NewStage()

	g := &generator{
		blocks:      make(map[string]*blocks.Block),
		definitions: definitions,
		lines:       lines,
	}

	errs := make([]error, 0)
	for _, stmt := range statements {
		err := stmt.Accept(g)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return g.blocks, errs
}

type generator struct {
	blocks map[string]*blocks.Block
	parent string
	lines  [][]rune

	blockID string

	variableInitializer *blocks.Block
	definitions         analyzer.Definitions

	noNext         bool
	variableName   string
	variableIsList bool

	warnings []error

	currentFunction *analyzer.Function
}

func (g *generator) VisitVarDecl(stmt *parser.StmtVarDecl) error {
	return nil
}

func (g *generator) VisitConstDecl(stmt *parser.StmtConstDecl) error {
	return nil
}

func (g *generator) VisitFuncDecl(stmt *parser.StmtFuncDecl) error {
	fn := g.definitions.Functions[stmt.Name.Lexeme]
	block := blocks.NewBlockTopLevel(blocks.ProceduresDefinition)
	g.blocks[block.ID] = block
	g.parent = block.ID

	g.noNext = true
	prototype := g.NewBlock(blocks.ProceduresPrototype, true)

	argumentNames := make([]string, 0, len(stmt.Params))
	argumentDefaults := make([]string, 0, len(stmt.Params))
	for _, p := range stmt.Params {
		if slices.Contains(argumentNames, p.Name.Lexeme) {
			return g.newError("Duplicate parameter name.", p.Name)
		}

		id := uuid.NewString()
		argumentNames = append(argumentNames, p.Name.Lexeme)
		argumentDefaults = append(argumentDefaults, "todo")

		g.noNext = true
		var reporterBlock *blocks.Block
		if p.Type.DataType == parser.DTBool {
			reporterBlock = g.NewBlock(blocks.ArgumentReporterBoolean, true)
		} else {
			reporterBlock = g.NewBlock(blocks.ArgumentReporterStringNumber, true)
		}
		reporterBlock.Fields["VALUE"] = []any{p.Name.Lexeme, nil}
		prototype.Inputs[id] = []any{1, reporterBlock.ID}
	}

	prototype.Mutation = map[string]any{
		"tagName":          "mutation",
		"children":         []any{},
		"proccode":         fn.ProcCode,
		"warp":             "false",
		"argumentids":      "[]",
		"argumentnames":    "[]",
		"argumentdefaults": "[]",
	}
	if len(stmt.Params) > 0 {
		prototype.Mutation["argumentids"] = fmt.Sprintf("[\"%s\"]", strings.Join(fn.ArgumentIDs, "\",\""))
		prototype.Mutation["argumentnames"] = fmt.Sprintf("[\"%s\"]", strings.Join(argumentNames, "\",\""))
		prototype.Mutation["argumentdefaults"] = fmt.Sprintf("[\"%s\"]", strings.Join(argumentDefaults, "\",\""))
	}

	g.definitions.Functions[stmt.Name.Lexeme] = &analyzer.Function{
		Name:        stmt.Name,
		Params:      stmt.Params,
		ProcCode:    fn.ProcCode,
		ArgumentIDs: fn.ArgumentIDs,
		StartLine:   stmt.StartLine,
		EndLine:     stmt.EndLine,
	}

	block.Inputs["custom_block"] = []any{1, prototype.ID}

	g.currentFunction = g.definitions.Functions[stmt.Name.Lexeme]
	g.parent = block.ID
	for _, s := range stmt.Body {
		err := s.Accept(g)
		if err != nil {
			return err
		}
		g.parent = g.blockID
	}
	g.currentFunction = nil

	return nil
}

func (g *generator) VisitEvent(stmt *parser.StmtEvent) error {
	ev := Events[stmt.Name.Lexeme]
	block, err := ev.Fn(g, stmt)
	if err != nil {
		return err
	}
	g.blocks[block.ID] = block
	g.parent = block.ID
	for _, s := range stmt.Body {
		err = s.Accept(g)
		if err != nil {
			return err
		}
		g.parent = g.blockID
	}
	return nil
}

func (g *generator) VisitFuncCall(stmt *parser.StmtFuncCall) error {
	if f, ok := g.definitions.Functions[stmt.Name.Lexeme]; ok {
		block := g.NewBlock(blocks.ProceduresCall, false)

		var err error
		for i, p := range stmt.Parameters {
			block.Inputs[f.ArgumentIDs[i]], err = g.value(block.ID, stmt.Name, p)
			if err != nil {
				return err
			}
		}

		block.Mutation = map[string]any{
			"tagName":     "mutation",
			"children":    []any{},
			"proccode":    f.ProcCode,
			"argumentids": "[]",
			"warp":        "false",
		}

		if len(f.Params) > 0 {
			block.Mutation["argumentids"] = fmt.Sprintf("[\"%s\"]", strings.Join(f.ArgumentIDs, "\",\""))
		}

		g.blockID = block.ID
	} else {
		fn := FuncCalls[stmt.Name.Lexeme]
		block, err := fn.Fn(g, stmt)
		if err != nil {
			return err
		}
		g.blockID = block.ID
	}

	return nil
}

func (g *generator) VisitAssignment(stmt *parser.StmtAssignment) error {
	var block *blocks.Block
	if assignment, ok := Assignments[stmt.Variable.Lexeme]; ok {
		blockType := assignment.AssignType
		if stmt.Operator.Type == parser.TkPlusAssign {
			blockType = assignment.IncreaseType
		}

		block = g.NewBlock(blockType, false)
		value, err := g.value(block.ID, stmt.Operator, stmt.Value)
		if err != nil {
			return err
		}
		block.Inputs[assignment.InputName] = value
	} else {
		variable := g.definitions.Variables[stmt.Variable.Lexeme]
		block = g.NewBlock(blocks.VariableSetTo, false)

		value, err := g.value(block.ID, stmt.Operator, stmt.Value)
		if err != nil {
			return err
		}
		block.Fields["VARIABLE"] = []any{variable.Name.Lexeme, variable.ID}

		if stmt.Operator.Type != parser.TkAssign {
			if variable.DataType == parser.DTNumber {
				block.Type = blocks.VariableChangeBy
			} else {
				g.noNext = true
				g.parent = block.ID
				joinBlock := g.NewBlock(blocks.OpJoin, false)
				joinBlock.Inputs["STRING1"] = []any{3, []any{12, variable.Name.Lexeme, variable.ID}, []any{10, ""}}
				joinBlock.Inputs["STRING2"] = value
				value = []any{3, joinBlock.ID, []any{10, ""}}
			}
		}

		block.Inputs["VALUE"] = value
	}

	g.blockID = block.ID
	return nil
}

func (g *generator) VisitIf(stmt *parser.StmtIf) error {
	var block *blocks.Block
	if stmt.ElseBody == nil {
		block = g.NewBlock(blocks.ControlIf, false)
	} else {
		block = g.NewBlock(blocks.ControlIfElse, false)
	}

	g.parent = block.ID

	g.noNext = true
	err := stmt.Condition.Accept(g)
	if err != nil {
		return err
	}
	block.Inputs["CONDITION"] = []any{2, g.blockID}

	g.noNext = true
	for i, s := range stmt.Body {
		err = s.Accept(g)
		if err != nil {
			return err
		}
		if i == 0 {
			block.Inputs["SUBSTACK"] = []any{2, g.blockID}
		}
		g.parent = g.blockID
	}

	g.noNext = true
	for i, s := range stmt.ElseBody {
		err = s.Accept(g)
		if err != nil {
			return err
		}
		if i == 0 {
			block.Inputs["SUBSTACK2"] = []any{2, g.blockID}
		}
		g.parent = g.blockID
	}
	g.noNext = false

	g.blockID = block.ID
	return nil
}

func (g *generator) VisitLoop(stmt *parser.StmtLoop) error {
	var block *blocks.Block
	var err error
	parent := g.parent
	if stmt.Condition == nil {
		block = g.NewBlock(blocks.ControlRepeatForever, false)
	} else if stmt.Keyword.Type == parser.TkWhile {
		block = g.NewBlock(blocks.ControlRepeatUntil, false)
		g.parent = block.ID
		block.Inputs["CONDITION"], err = g.value(parent, stmt.Keyword, stmt.Condition)
		if err != nil {
			return err
		}
	} else if stmt.Keyword.Type == parser.TkFor {
		block = g.NewBlock(blocks.ControlRepeat, false)
		g.parent = block.ID
		block.Inputs["TIMES"], err = g.value(parent, stmt.Keyword, stmt.Condition)
		if err != nil {
			return err
		}
	} else {
		return g.newError("Unknown loop type.", stmt.Keyword)
	}
	g.parent = block.ID
	g.noNext = true
	for i, s := range stmt.Body {
		err = s.Accept(g)
		if err != nil {
			return err
		}
		if i == 0 {
			block.Inputs["SUBSTACK"] = []any{2, g.blockID}
		}
		g.parent = g.blockID
	}
	g.noNext = false
	g.blockID = block.ID
	return nil
}

func (g *generator) VisitIdentifier(expr *parser.ExprIdentifier) error {
	if g.currentFunction != nil {
		for _, p := range g.currentFunction.Params {
			if p.Name.Lexeme == expr.Name.Lexeme {
				var reporterBlock *blocks.Block
				g.noNext = true
				if p.Type.DataType == parser.DTBool {
					reporterBlock = g.NewBlock(blocks.ArgumentReporterBoolean, false)
				} else {
					reporterBlock = g.NewBlock(blocks.ArgumentReporterStringNumber, false)
				}
				reporterBlock.Fields["VALUE"] = []any{p.Name.Lexeme, nil}
				g.blockID = reporterBlock.ID
				return nil
			}
		}
	}

	if v, ok := Variables[expr.Name.Lexeme]; ok {
		block := g.NewBlock(v.blockType, false)
		if v.fields != nil {
			block.Fields = v.fields
		}
		if v.fn != nil {
			v.fn(g, block)
		}
		g.blockID = block.ID
		return nil
	}

	if variable, ok := g.definitions.Variables[expr.Name.Lexeme]; ok {
		g.variableName = variable.Name.Lexeme
		return nil
	}

	if l, ok := g.definitions.Lists[expr.Name.Lexeme]; ok {
		g.variableName = l.Name.Lexeme
		g.variableIsList = true
		return nil
	}

	if _, ok := g.definitions.Constants[expr.Name.Lexeme]; ok {
		return g.newError("Constants are not allowed in this context.", expr.Name)
	}

	return g.newError("Unknown identifier.", expr.Name)
}

func (g *generator) VisitExprFuncCall(expr *parser.ExprFuncCall) error {
	fn, ok := ExprFuncCalls[expr.Name.Lexeme]
	if !ok {
		if _, ok := FuncCalls[expr.Name.Lexeme]; ok {
			return g.newError("Only functions which return a value are allowed in this context.", expr.Name)
		}
		return g.newError("Unknown function.", expr.Name)
	}
	block, err := fn.Fn(g, expr)
	if err != nil {
		return err
	}
	g.blockID = block.ID
	return nil
}

func (g *generator) VisitTypeCast(expr *parser.ExprTypeCast) error {
	return expr.Value.Accept(g)
}

func (g *generator) VisitLiteral(expr *parser.ExprLiteral) error {
	return g.newError("Literals are not allowed in this context.", expr.Token)
}

func (g *generator) VisitListInitializer(expr *parser.ExprListInitializer) error {
	return g.newError("Literals are not allowed in this context.", expr.OpenBracket)
}

func (g *generator) VisitUnary(expr *parser.ExprUnary) error {
	var block *blocks.Block
	switch expr.Operator.Type {
	case parser.TkBang:
		block = g.NewBlock(blocks.OpNot, false)
	}
	g.parent = block.ID
	input, err := g.value(g.parent, expr.Operator, expr.Right)
	if err != nil {
		return err
	}
	block.Inputs["OPERAND"] = input

	block.Next = nil
	g.blockID = block.ID
	return nil
}

func (g *generator) VisitBinary(expr *parser.ExprBinary) error {
	var block *blocks.Block
	if expr.Operator.Type == parser.TkPlus || expr.Operator.Type == parser.TkEqual {
		block = g.NewBlock(blocks.OpAdd, false)

		left, err := g.value(block.ID, expr.Operator, expr.Left)
		if err != nil {
			return err
		}

		right, err := g.value(block.ID, expr.Operator, expr.Right)
		if err != nil {
			return err
		}

		if expr.Operator.Type == parser.TkEqual {
			block.Inputs["OPERAND1"] = left
			block.Inputs["OPERAND2"] = right
			block.Type = blocks.OpEquals
		} else {
			if expr.Left.Type() == parser.DTString || expr.Right.Type() == parser.DTString {
				block.Type = blocks.OpJoin
				block.Inputs["STRING1"] = left
				block.Inputs["STRING2"] = right
			} else {
				block.Inputs["NUM1"] = left
				block.Inputs["NUM2"] = right
			}
		}
	} else {
		operandName := "OPERAND"
		switch expr.Operator.Type {
		case parser.TkLess:
			block = g.NewBlock(blocks.OpLessThan, false)
		case parser.TkGreater:
			block = g.NewBlock(blocks.OpGreaterThan, false)
		case parser.TkAnd:
			block = g.NewBlock(blocks.OpAnd, false)
		case parser.TkOr:
			block = g.NewBlock(blocks.OpOr, false)
		default:
			operandName = "NUM"
			switch expr.Operator.Type {
			case parser.TkMinus:
				block = g.NewBlock(blocks.OpSubtract, false)
			case parser.TkMultiply:
				block = g.NewBlock(blocks.OpMultiply, false)
			case parser.TkDivide:
				block = g.NewBlock(blocks.OpDivide, false)
			case parser.TkModulus:
				block = g.NewBlock(blocks.OpMod, false)
			default:
				return g.newError("Unknown binary operator.", expr.Operator)
			}
		}

		left, err := g.value(block.ID, expr.Operator, expr.Left)
		if err != nil {
			return err
		}
		block.Inputs[operandName+"1"] = left

		right, err := g.value(block.ID, expr.Operator, expr.Right)
		if err != nil {
			return err
		}
		block.Inputs[operandName+"2"] = right
	}

	g.blockID = block.ID
	return nil
}

var matchAllRegex = regexp.MustCompile(".*")

func (g *generator) value(parent string, token parser.Token, expr parser.Expr) ([]any, error) {
	return g.valueWithRegex(parent, token, expr, matchAllRegex, -1, "")
}

func (g *generator) valueWithRegex(parent string, token parser.Token, expr parser.Expr, validate *regexp.Regexp, valueIntOverride int, errorMessage string) ([]any, error) {
	return g.valueWithValidator(parent, token, expr, func(v any) bool {
		return validate.MatchString(fmt.Sprintf("%v", v))
	}, valueIntOverride, errorMessage)
}

func (g *generator) valueInRange(parent string, token parser.Token, expr parser.Expr, valueIntOverride int, min any, max any) ([]any, error) {
	return g.valueWithValidator(parent, token, expr, func(v any) bool {
		switch value := v.(type) {
		case string:
			return value >= min.(string) && value <= max.(string)
		case float64:
			if _, ok := min.(int); ok {
				return int(value) >= min.(int) && int(value) <= max.(int)
			}
			return value >= min.(float64) && value <= max.(float64)
		}
		return false
	}, valueIntOverride, fmt.Sprintf("The value must lie between %v and %v.", min, max))
}

func (g *generator) valueWithValidator(parent string, token parser.Token, expr parser.Expr, validate func(v any) bool, valueIntOverride int, errorMessage string) ([]any, error) {
	var castType parser.Token
	castValue := expr
	if cast, ok := expr.(*parser.ExprTypeCast); ok {
		castType = cast.Target
		castValue = cast.Value
	}

	if literalExpr, ok := castValue.(*parser.ExprLiteral); ok {
		literal := *literalExpr
		if castValue != expr {
			literal.Token = castToken(literal.Token, castType.DataType)
		}
		if literal.Token.DataType == parser.DTBool {
			return nil, g.newError("Boolean literals are not allowed in this context.", literal.Token)
		}
		if !validate(literal.Token.Literal) {
			return nil, g.newError(errorMessage, literal.Token)
		}
		return []any{1, []any{intFromDT(literal.Token.DataType, valueIntOverride), fmt.Sprintf("%v", literal.Token.Literal)}}, nil
	} else {
		if ident, ok := castValue.(*parser.ExprIdentifier); ok {
			if myConst, ok := g.definitions.Constants[ident.Name.Lexeme]; ok {
				constant := *myConst
				if castValue != expr {
					constant.Type = castType.DataType
					constant.Value = castToken(constant.Value, castType.DataType)
				}
				if constant.Type == parser.DTBool {
					return nil, g.newError("Boolean constants are not allowed in this context.", ident.Name)
				}
				if !validate(constant.Value.Literal) {
					return nil, g.newError(errorMessage, ident.Name)
				}
				return []any{1, []any{intFromDT(constant.Type, valueIntOverride), fmt.Sprintf("%v", constant.Value.Literal)}}, nil
			}
		}
		g.parent = parent
		g.noNext = true
		defer func() { g.variableName = ""; g.variableIsList = false }()
		err := expr.Accept(g)
		if err != nil {
			return nil, err
		}
		g.noNext = false
		if expr.Type() == parser.DTBool {
			return []any{2, g.blockID}, nil
		}
		if g.variableName != "" {
			if g.variableIsList {
				list := g.definitions.Lists[g.variableName]
				return []any{3, []any{13, list.Name.Lexeme, list.ID}, []any{intFromDT(expr.Type(), valueIntOverride), ""}}, nil
			}
			variable := g.definitions.Variables[g.variableName]
			return []any{3, []any{12, variable.Name.Lexeme, variable.ID}, []any{intFromDT(expr.Type(), valueIntOverride), ""}}, nil
		}
		return []any{3, g.blockID, []any{intFromDT(expr.Type(), valueIntOverride), ""}}, nil
	}
}

func intFromDT(dataType parser.DataType, valueIntOverride int) int {
	if valueIntOverride != -1 {
		return valueIntOverride
	}
	switch dataType {
	case parser.DTString:
		return 10
	}
	return 4
}

func (g *generator) fieldMenu(blockType blocks.BlockType, surroundStringsWith, menuFieldKey string, parent string, token parser.Token, expr parser.Expr, validateValue func(v any, token parser.Token) error) ([]any, error) {
	var castType parser.Token
	castValue := expr
	if cast, ok := expr.(*parser.ExprTypeCast); ok {
		castType = cast.Target
		castValue = cast.Value
	}

	gparent := g.parent
	defer func() { g.parent = gparent }()
	g.parent = parent
	g.noNext = true
	defer func() { g.variableName = ""; g.noNext = false; g.variableIsList = false }()
	if literalExpr, ok := castValue.(*parser.ExprLiteral); ok {
		literal := *literalExpr
		if castValue != expr {
			literal.Token = castToken(literal.Token, castType.DataType)
		}
		if literal.Token.DataType == parser.DTBool {
			return nil, g.newError("Boolean literals are not allowed in this context.", literal.Token)
		}

		if err := validateValue(literal.Token.Literal, literal.Token); err != nil {
			return nil, err
		}

		block := g.NewBlock(blockType, true)

		value := fmt.Sprintf("%v", literal.Token.Literal)
		if _, ok := literal.Token.Literal.(string); ok {
			value = fmt.Sprintf("%s%s%s", surroundStringsWith, literal.Token.Literal, surroundStringsWith)
		}

		block.Fields[menuFieldKey] = []any{value, nil}
		return []any{1, block.ID}, nil
	} else {
		if ident, ok := castValue.(*parser.ExprIdentifier); ok {
			if myConst, ok := g.definitions.Constants[ident.Name.Lexeme]; ok {
				constant := *myConst
				if castValue != expr {
					constant.Type = castType.DataType
					constant.Value = castToken(constant.Value, castType.DataType)
				}
				if constant.Type == parser.DTBool {
					return nil, g.newError("Boolean constants are not allowed in this context.", ident.Name)
				}
				if err := validateValue(constant.Value.Literal, ident.Name); err != nil {
					return nil, err
				}

				block := g.NewBlock(blockType, true)

				value := fmt.Sprintf("%v", constant.Value.Literal)
				if _, ok := constant.Value.Literal.(string); ok {
					value = fmt.Sprintf("%s%s%s", surroundStringsWith, constant.Value.Literal, surroundStringsWith)
				}

				block.Fields[menuFieldKey] = []any{value, nil}
				return []any{1, block.ID}, nil
			}
		}
		block := g.NewBlock(blockType, true)
		block.Fields[menuFieldKey] = []any{"", nil}
		g.noNext = true
		err := expr.Accept(g)
		if err != nil {
			return nil, err
		}
		if g.variableName != "" {
			if g.variableIsList {
				list := g.definitions.Lists[g.variableName]
				return []any{3, []any{13, list.Name.Lexeme, list.ID}, block.ID}, nil
			}
			variable := g.definitions.Variables[g.variableName]
			return []any{3, []any{12, variable.Name.Lexeme, variable.ID}, block.ID}, nil
		}
		return []any{3, g.blockID, block.ID}, nil
	}
}

func (g *generator) literal(token parser.Token, expr parser.Expr) (any, error) {
	var castType parser.Token
	castValue := expr
	if cast, ok := expr.(*parser.ExprTypeCast); ok {
		castType = cast.Target
		castValue = cast.Value
	}

	if literalExpr, ok := castValue.(*parser.ExprLiteral); ok {
		literal := *literalExpr
		if castValue != expr {
			literal.Token = castToken(literal.Token, castType.DataType)
		}
		return literal.Token.Literal, nil
	}
	if ident, ok := castValue.(*parser.ExprIdentifier); ok {
		if myConst, ok := g.definitions.Constants[ident.Name.Lexeme]; ok {
			constant := *myConst
			if castValue != expr {
				constant.Type = castType.DataType
				constant.Value = castToken(constant.Value, castType.DataType)
			}
			return constant.Value.Literal, nil
		}
	}
	return nil, g.newError("Only literals are allowed in this context.", token)
}

func (g *generator) NewBlock(blockType blocks.BlockType, shadow bool) *blocks.Block {
	var block *blocks.Block
	if shadow {
		block = blocks.NewShadowBlock(blockType, g.parent)
	} else {
		block = blocks.NewBlock(blockType, g.parent)
	}
	g.blocks[block.ID] = block
	parent := g.blocks[g.parent]
	if !g.noNext && !parent.NoNext {
		parent.Next = &block.ID
	}
	g.noNext = false
	return block
}

func castToken(token parser.Token, dataType parser.DataType) parser.Token {
	switch dataType {
	case parser.DTBool:
		switch token.DataType {
		case parser.DTNumber:
			if token.Literal.(float64) == 0 {
				token.Literal = false
			} else {
				token.Literal = true
			}
		case parser.DTString:
			token.Literal, _ = strconv.ParseBool(token.Literal.(string))
		}
	case parser.DTNumber:
		switch token.DataType {
		case parser.DTBool:
			if token.Literal.(bool) {
				token.Literal = 1
			} else {
				token.Literal = 0
			}
		case parser.DTString:
			token.Literal, _ = strconv.ParseFloat(token.Literal.(string), 64)
		}
	case parser.DTString:
		switch token.DataType {
		case parser.DTBool:
			token.Literal = fmt.Sprintf("%t", token.Literal.(bool))
		case parser.DTNumber:
			token.Literal = fmt.Sprintf("%v", token.Literal)
		}
	}
	token.DataType = dataType
	return token
}

type GenerateError struct {
	Token   parser.Token
	Message string
	Line    []rune
	Warning bool
}

func (p GenerateError) Error() string {
	length := len([]rune(p.Token.Lexeme))
	if p.Token.Type == parser.TkNewLine {
		length = 1
	}
	if p.Warning {
		return generateWarningText(p.Message, p.Line, p.Token.Line, p.Token.Column, p.Token.Column+length)
	} else {
		return generateErrorText(p.Message, p.Line, p.Token.Line, p.Token.Column, p.Token.Column+length)
	}
}

func (g *generator) newError(message string, token parser.Token) error {
	return GenerateError{
		Token:   token,
		Message: message,
		Line:    g.lines[token.Line],
	}
}

func (g *generator) newWarning(message string, token parser.Token) {
	g.warnings = append(g.warnings, GenerateError{
		Token:   token,
		Message: message,
		Line:    g.lines[token.Line],
		Warning: true,
	})
}
