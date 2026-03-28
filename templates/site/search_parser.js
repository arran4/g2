class SearchParser {
    constructor(query) {
        this.query = query || "";
        this.pos = 0;
        this.tokens = this.tokenize(this.query);
        this.tokenIndex = 0;
    }

    tokenize(query) {
        const tokens = [];
        let pos = 0;

        while (pos < query.length) {
            // Skip whitespace
            while (pos < query.length && /\s/.test(query[pos])) {
                pos++;
            }
            if (pos >= query.length) break;

            const char = query[pos];

            if (char === '(' || char === ')') {
                tokens.push({ type: 'PAREN', value: char });
                pos++;
            } else if (char === '-' || char === '!') {
                tokens.push({ type: 'NOT', value: char });
                pos++;
            } else if (char === '&' && query[pos + 1] === '&') {
                tokens.push({ type: 'AND', value: '&&' });
                pos += 2;
            } else if (char === '|' && query[pos + 1] === '|') {
                tokens.push({ type: 'OR', value: '||' });
                pos += 2;
            } else if (char === "'") {
                // Sequence start
                pos++;
                let seq = "";
                while (pos < query.length && query[pos] !== "'") {
                    seq += query[pos];
                    pos++;
                }
                if (pos < query.length && query[pos] === "'") {
                    pos++; // Consume closing quote
                }
                tokens.push({ type: 'SEQUENCE', value: seq });
            } else {
                // Regular term or field
                let term = "";
                let inQuotes = false;

                while (pos < query.length) {
                    if (query[pos] === '"') {
                        inQuotes = !inQuotes;
                        pos++;
                        continue;
                    }
                    if (!inQuotes && /\s/.test(query[pos])) break;
                    if (!inQuotes && (query[pos] === '(' || query[pos] === ')')) break;

                    term += query[pos];
                    pos++;
                }

                if (term === "AND") {
                    tokens.push({ type: 'AND', value: term });
                } else if (term === "OR") {
                    tokens.push({ type: 'OR', value: term });
                } else if (term === "NOT") {
                    tokens.push({ type: 'NOT', value: term });
                } else {
                    tokens.push({ type: 'TERM', value: term });
                }
            }
        }
        return tokens;
    }

    peek() {
        return this.tokens[this.tokenIndex];
    }

    consume() {
        return this.tokens[this.tokenIndex++];
    }

    parse() {
        if (this.tokens.length === 0) return null;
        const ast = this.parseOr();
        return ast;
    }

    parseOr() {
        let left = this.parseAnd();
        while (this.peek() && this.peek().type === 'OR') {
            this.consume();
            let right = this.parseAnd();
            left = { type: 'OR', left: left, right: right };
        }
        return left;
    }

    parseAnd() {
        let left = this.parseUnary();
        while (this.peek() && this.peek().type !== 'OR' && this.peek().value !== ')') {
            // Implicit AND if the next token is a TERM, NOT, PAREN, or explicit AND
            if (this.peek().type === 'AND') {
                this.consume();
            }
            let right = this.parseUnary();
            left = { type: 'AND', left: left, right: right };
        }
        return left;
    }

    parseUnary() {
        if (this.peek() && this.peek().type === 'NOT') {
            this.consume();
            let expr = this.parsePrimary();
            return { type: 'NOT', expr: expr };
        }
        return this.parsePrimary();
    }

    parsePrimary() {
        const token = this.peek();
        if (!token) return null;

        if (token.type === 'PAREN' && token.value === '(') {
            this.consume();
            let expr = this.parseOr();
            if (this.peek() && this.peek().type === 'PAREN' && this.peek().value === ')') {
                this.consume();
            }
            return { type: 'GROUP', expr: expr };
        }

        if (token.type === 'SEQUENCE') {
            this.consume();
            return { type: 'SEQUENCE', value: token.value };
        }

        if (token.type === 'TERM') {
            this.consume();

            // Handle field prefix
            const colonIndex = token.value.indexOf(':');
            if (colonIndex > 0) {
                const field = token.value.substring(0, colonIndex);
                const val = token.value.substring(colonIndex + 1);
                return { type: 'FIELD', field: field.toLowerCase(), value: val };
            }

            return { type: 'TERM', value: token.value };
        }

        // If we hit something unexpected, just consume it as a generic term to prevent infinite loops
        this.consume();
        return { type: 'TERM', value: token.value };
    }
}
