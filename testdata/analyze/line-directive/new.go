package parser

//line parser.y:100
func (p *Parser) Pos() int { return p.pos }

//line parser.y:200
func (p *Parser) Tok() int { return p.tok }

func (p *Parser) next() error {
	err := p.scan()
	if err != nil {
		return err
	}
	return nil
}
