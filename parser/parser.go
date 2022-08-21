package parser

import "fmt"

type parser struct {
	tokens  []Token
	current int
	lines   [][]rune
	errors  []error
}

func Parse(tokens []Token, lines [][]rune) ([]Stmt, []error) {
	parser := &parser{
		tokens: tokens,
		lines:  lines,
		errors: make([]error, 0),
	}
	return parser.parse()
}

func (p *parser) parse() ([]Stmt, []error) {
	statements := make([]Stmt, 0)
	for p.peek().Type != TkEOF {
		statements = append(statements, p.topLevel())
	}
	return statements, p.errors
}

func (p *parser) topLevel() Stmt {
	stmt, err := p.event()
	if err != nil {
		p.errors = append(p.errors, err)
		p.synchronize()
	}
	return stmt
}

func (p *parser) event() (Stmt, error) {
	if !p.match(TkAt) {
		return nil, p.newError("Expected event.")
	}

	if !p.match(TkIdentifier) {
		return nil, p.newError("Expected event name after '@'.")
	}
	name := p.previous()

	if !p.match(TkOpenParen) {
		return nil, p.newError("Expected '(' after event name.")
	}

	var parameter Token
	if p.match(TkLiteral) {
		parameter = p.previous()
	}

	if !p.match(TkCloseParen) {
		return nil, p.newError("Expected ')' after parameter.")
	}

	if !p.match(TkColon) {
		return nil, p.newError("Expected ':' after parameter.")
	}

	if !p.match(TkNewLine) {
		return nil, p.newError("Expected '\n' after ':'.")
	}

	body := p.statements(1)

	return &StmtEvent{
		Name:      name,
		Parameter: parameter,
		Body:      body,
	}, nil
}

func (p *parser) statements(indent int) []Stmt {
	statements := make([]Stmt, 0, 10)
	for p.peek().Indent >= indent {
		stmt, err := p.statement(indent)
		if err == nil {
			statements = append(statements, stmt)
		} else {
			p.errors = append(p.errors, err)
			p.synchronize()
		}
	}
	return statements
}

func (p *parser) statement(indent int) (Stmt, error) {
	switch p.peek().Type {
	case TkIf:
		return p.ifStmt(indent)
	case TkWhile:
		return p.whileLoop(indent)
	case TkFor:
		return p.forLoop(indent)
	}

	if p.peekNext().Type == TkOpenParen {
		return p.funcCall()
	} else if p.peekNext().Type == TkAssign || p.peekNext().Type == TkPlusAssign || p.peekNext().Type == TkMinusAssign || p.peekNext().Type == TkMultiplyAssign || p.peekNext().Type == TkDivideAssign {
		return p.assignment()
	}

	return nil, p.newError("Expected statement.")
}

func (p *parser) funcCall() (Stmt, error) {
	if !p.match(TkIdentifier) {
		return nil, p.newError("Expected identifier.")
	}
	name := p.previous()

	if !p.match(TkOpenParen) {
		return nil, p.newError("Expected '(' after identifier.")
	}

	parameters := make([]Expr, 0, 1)
	for p.peek().Type != TkCloseParen && p.peek().Type != TkEOF {
		param, err := p.expression()
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, param)
		if p.peek().Type != TkComma {
			break
		}
	}

	if !p.match(TkCloseParen) {
		return nil, p.newError("Expected ')' after parameter list.")
	}

	if !p.match(TkNewLine) {
		return nil, p.newError("Expected '\n' after statement.")
	}

	return &StmtFuncCall{
		Name:       name,
		Parameters: parameters,
	}, nil
}

func (p *parser) assignment() (Stmt, error) {
	if !p.match(TkIdentifier) {
		return nil, p.newError("Expected identifier.")
	}
	variable := p.previous()

	if !p.match(TkAssign, TkPlusAssign, TkMinusAssign, TkMultiplyAssign, TkDivideAssign) {
		return nil, p.newError("Expected assignment operator after identifier.")
	}
	operator := p.previous()

	value, err := p.expression()
	if err != nil {
		return nil, err
	}

	if !p.match(TkNewLine) {
		return nil, p.newError("Expected '\n' after statement.")
	}

	return &StmtAssignment{
		Variable: variable,
		Operator: operator,
		Value:    value,
	}, nil
}

func (p *parser) ifStmt(indent int) (Stmt, error) {
	if !p.match(TkIf) {
		return nil, p.newError("Expected 'if' keyword.")
	}
	keyword := p.previous()

	condition, err := p.expression()
	if err != nil {
		return nil, err
	}

	if !p.match(TkColon) {
		return nil, p.newError("Expected ':' after if condition.")
	}

	if !p.match(TkNewLine) {
		return nil, p.newError("Expected '\\n' after ':'.")
	}

	body := p.statements(indent + 1)

	var elseBody []Stmt
	if p.match(TkElse) {
		elseBody = p.statements(indent + 1)
	}

	return &StmtIf{
		Keyword:   keyword,
		Condition: condition,
		Body:      body,
		ElseBody:  elseBody,
	}, nil
}

func (p *parser) whileLoop(indent int) (Stmt, error) {
	if !p.match(TkWhile) {
		return nil, p.newError("Expected 'while' keyword.")
	}
	keyword := p.previous()

	condition, err := p.expression()
	if err != nil {
		return nil, err
	}

	if !p.match(TkColon) {
		return nil, p.newError("Expected ':' after while condition.")
	}

	if !p.match(TkNewLine) {
		return nil, p.newError("Expected '\\n' after ':'.")
	}

	body := p.statements(indent + 1)

	return &StmtLoop{
		Keyword:   keyword,
		Condition: condition,
		Body:      body,
	}, nil
}

func (p *parser) forLoop(indent int) (Stmt, error) {
	if !p.match(TkFor) {
		return nil, p.newError("Expected 'for' keyword.")
	}
	keyword := p.previous()

	condition, err := p.expression()
	if err != nil {
		return nil, err
	}

	if !p.match(TkColon) {
		return nil, p.newError("Expected ':' after for condition.")
	}

	if !p.match(TkNewLine) {
		return nil, p.newError("Expected '\\n' after ':'.")
	}

	body := p.statements(indent + 1)

	return &StmtLoop{
		Keyword:   keyword,
		Condition: condition,
		Body:      body,
	}, nil
}

func (p *parser) expression() (Expr, error) {
	return p.or()
}

func (p *parser) or() (Expr, error) {
	expr, err := p.and()
	if err != nil {
		return nil, err
	}

	for p.match(TkOr) {
		operator := p.previous()
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		expr = &ExprBinary{
			Operator: operator,
			Left:     expr,
			Right:    right,
		}
	}

	return expr, nil
}

func (p *parser) and() (Expr, error) {
	expr, err := p.equality()
	if err != nil {
		return nil, err
	}

	for p.match(TkAnd) {
		operator := p.previous()
		right, err := p.equality()
		if err != nil {
			return nil, err
		}
		expr = &ExprBinary{
			Operator: operator,
			Left:     expr,
			Right:    right,
		}
	}

	return expr, nil
}

func (p *parser) equality() (Expr, error) {
	expr, err := p.comparison()
	if err != nil {
		return nil, err
	}

	for p.match(TkEqual, TkNotEqual) {
		operator := p.previous()
		right, err := p.comparison()
		if err != nil {
			return nil, err
		}
		expr = &ExprBinary{
			Operator: operator,
			Left:     expr,
			Right:    right,
		}
	}

	return expr, nil
}

func (p *parser) comparison() (Expr, error) {
	expr, err := p.term()
	if err != nil {
		return nil, err
	}

	for p.match(TkLess, TkLessEqual, TkGreater, TkGreaterEqual) {
		operator := p.previous()
		right, err := p.term()
		if err != nil {
			return nil, err
		}
		expr = &ExprBinary{
			Operator: operator,
			Left:     expr,
			Right:    right,
		}
	}

	return expr, nil
}

func (p *parser) term() (Expr, error) {
	expr, err := p.factor()
	if err != nil {
		return nil, err
	}

	for p.match(TkPlus, TkMinus) {
		operator := p.previous()
		right, err := p.factor()
		if err != nil {
			return nil, err
		}
		expr = &ExprBinary{
			Operator: operator,
			Left:     expr,
			Right:    right,
		}
	}

	return expr, nil
}

func (p *parser) factor() (Expr, error) {
	expr, err := p.unary()
	if err != nil {
		return nil, err
	}

	for p.match(TkMultiply, TkDivide) {
		operator := p.previous()
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		expr = &ExprBinary{
			Operator: operator,
			Left:     expr,
			Right:    right,
		}
	}

	return expr, nil
}

func (p *parser) unary() (Expr, error) {
	if p.match(TkBang, TkMinus) {
		operator := p.previous()
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &ExprUnary{
			Operator: operator,
			Right:    right,
		}, nil
	}

	return p.primary()
}

func (p *parser) primary() (Expr, error) {
	if p.match(TkIdentifier) {
		return &ExprIdentifier{
			Name: p.previous(),
		}, nil
	}

	if p.match(TkLiteral) {
		return &ExprLiteral{
			Value: p.previous().Literal,
		}, nil
	}

	return nil, p.newError(fmt.Sprintf("Unexpected token '%s'", p.peek().Lexeme))
}

func (p *parser) match(types ...TokenType) bool {
	for _, t := range types {
		if p.peek().Type == t {
			p.current++
			return true
		}
	}
	return false
}

func (p *parser) previous() Token {
	return p.tokens[p.current-1]
}

func (p *parser) peek() Token {
	return p.tokens[p.current]
}

func (p *parser) peekNext() Token {
	return p.tokens[p.current+1]
}

func (p *parser) synchronize() {
	if p.peek().Type == TkEOF {
		return
	}
	p.current++
	for p.peek().Type != TkEOF {
		switch p.peek().Type {
		case TkNewLine:
			p.current++
			return
		}
		p.current++
	}
}

type ParseError struct {
	Token   Token
	Message string
	Line    []rune
}

func (p ParseError) Error() string {
	length := len([]rune(p.Token.Lexeme))
	if p.Token.Type == TkNewLine {
		length = 1
	}
	return generateErrorText(p.Message, p.Line, p.Token.Line, p.Token.Column, p.Token.Column+length)
}

func (p *parser) newError(message string) error {
	return ParseError{
		Token:   p.peek(),
		Message: message,
		Line:    p.lines[p.peek().Line],
	}
}

func (p *parser) newErrorAt(message string, token Token) error {
	return ParseError{
		Token:   token,
		Message: message,
		Line:    p.lines[token.Line],
	}
}
