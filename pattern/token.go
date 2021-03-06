package pattern

import (
	"fmt"
	"strings"
)

// Token is a type for specifying the tokens used in noise.
type Token string

const (
	// TokenE is the e from noise specs.
	TokenE = Token("e")
	// TokenS is the s from noise specs.
	TokenS = Token("s")
	// TokenEe is the ee from noise specs.
	TokenEe = Token("ee")
	// TokenEs is the es from noise specs.
	TokenEs = Token("es")
	// TokenSe is the se from noise specs.
	TokenSe = Token("se")
	// TokenSs is the ss from noise specs.
	TokenSs = Token("ss")
	// TokenPsk is the psk from noise specs.
	TokenPsk = Token("psk")

	// TokenInitiator indicates the message is sent from initiator to responder.
	TokenInitiator = Token("->")
	// TokenResponder indicates the message is sent from responder to initiator.
	TokenResponder = Token("<-")

	tokenInvalid        = Token("invalid")
	preMessageIndicator = "..."

	errConsecutiveTokens = "cannot have two consecutive line using %s"
	errRepeatedTokens    = "token '%s' appeared more than once"
	errMissingToken      = "need token %s before %s"
	errMustBeInitiator   = "the first line must be from initiator"
	errInvalidLine       = "line '%s' is invalid"
	errPskNotAllowed     = "psk is not allowed"
	errTooManyTokens     = "pre-message cannot have more then 2 tokens"
	errTokenNotAllowed   = "%s is not allowed in pre-message"
)

type patternLine []Token
type pattern []patternLine

func errInvalidPattern(format string, a ...interface{}) error {
	prefix := "Invalid pattern: "
	return fmt.Errorf(prefix+format, a...)
}

// parseMessageLine takes a line of messages, check its validation, and split it
// into a slice of token strings. For example,
// "-> e, s" becomes ["->", "e", "s"]
func parseMessageLine(l string) (patternLine, error) {
	pl := patternLine{}
	tokens := strings.Split(l, " ")

	// a valid line must have at least two items
	if len(tokens) < 2 {
		return nil, errInvalidPattern(errInvalidLine, l)
	}

	// the first item of a line must be a direction, left or right.
	t, err := parseTokenFromString(tokens[0])
	if err != nil {
		return nil, err
	}
	if t != TokenResponder && t != TokenInitiator {
		return nil, errInvalidPattern(errInvalidLine, l)
	}
	pl = append(pl, t)

	for _, token := range tokens[1:] {
		// "e," becomes "e"
		tokenTrimmed := strings.Trim(token, " ,")
		t, err := parseTokenFromString(tokenTrimmed)
		if err != nil {
			return nil, err
		}
		pl = append(pl, t)
	}

	return pl, nil
}

// parseTokenFromString turns a token string into a token type.
func parseTokenFromString(s string) (Token, error) {
	switch s {
	case "e":
		return TokenE, nil
	case "s":
		return TokenS, nil
	case "ee":
		return TokenEe, nil
	case "es":
		return TokenEs, nil
	case "se":
		return TokenSe, nil
	case "ss":
		return TokenSs, nil
	case "->":
		return TokenInitiator, nil
	case "<-":
		return TokenResponder, nil
	case "psk":
		return TokenPsk, nil
	default:
		return tokenInvalid, fmt.Errorf("token %s is invalid", s)
	}
}

// tokenize takes a message string and turns it into a pattern. For example, it
// takes,
//   -> e
//   <- e, ee
// and returns, a pattern, which is []patternline. A patternline is []Token.
func tokenize(ms string, pre bool) (pattern, error) {
	p := pattern{}

	// remove message whitespaces
	ms = strings.TrimSpace(ms)

	// break the message line by line, a message,
	//   -> e
	//   <- e, ee
	// becomes,
	// "-> e" and "<- e, ee"
	for _, line := range strings.Split(ms, "\n") {
		// remove line whitespaces
		line = strings.TrimSpace(line)

		// "<- e, ee" now becomes, "<-", "e", "ee"
		pl, err := parseMessageLine(line)
		if err != nil {
			return nil, err
		}
		p = append(p, pl)
	}

	// validate pattern based on it's pre-message or not
	if pre {
		if err := validatePrePattern(p); err != nil {
			return nil, err
		}
		return p, nil
	}

	if err := validatePattern(p); err != nil {
		return nil, err
	}

	return p, nil
}

// tokenizePreMessage takes a pre-message string and turns it into tokens. A
// valid pre-message must pass the following checks,
//  - it can only have a line of "e", "s", or "e, s", no "psk" is allowed.
func validatePrePattern(pl pattern) error {
	isInitiator := pl[0][0] == TokenInitiator
	prevIsInitiator := !isInitiator

	for _, line := range pl {
		isInitiator = line[0] == TokenInitiator
		// In additional to the rules specified in the noise protocol, it's also
		// required that the initiator/responder cannot send two consecutive
		// messages, they must alternate. For instance,
		//   -> s
		//   <- s
		// is a legal patter, while,
		//   -> s
		//   -> s
		// is not legal as they are both from the initiator(->)
		if prevIsInitiator == isInitiator {
			return errInvalidPattern(errConsecutiveTokens, line[0])
		}
		prevIsInitiator = isInitiator

		// pre-message can have at most 2 tokens, e and s, plus a direction
		// token, "->" or "<-", so max is 3.
		if len(line) > 3 {
			return errInvalidPattern(errTooManyTokens)
		}

		// check the tokens
		tokens := line[1:]
		if len(tokens) == 1 {
			t := tokens[0]
			switch t {
			case TokenE:
			case TokenS:
			default:
				return errInvalidPattern(errTokenNotAllowed, t)
			}
		}

		if len(tokens) == 2 {
			if tokens[0] != TokenE || tokens[1] != TokenS {
				return errInvalidPattern(errTokenNotAllowed, tokens)
			}
		}

	}
	return nil
}

// validatePattern implements the rules specified in the noise specs, which,
// 1. Parties must not send their static public key or ephemeral public key
// more than once per handshake.
// 2. Parties must not perform a DH calculation more than once per handshake
// (i.e. there must be no more than one occurrence of "ee", "es", "se", or
// "ss" per handshake).
// 3. After an "se" token, the initiator must not send a handshake payload or
// transport payload unless there has also been an "ee" token.
// 4. After an "ss" token, the initiator must not send a handshake payload or
// transport payload unless there has also been an "es" token.
// 5. After an "es" token, the responder must not send a handshake payload or
// transport payload unless there has also been an "ee" token.
// 6. After an "ss" token, the responder must not send a handshake payload or
// transport payload unless there has also been an "se" token.
func validatePattern(pl pattern) error {
	tokenSeen := map[Token]int{}

	// checks that the first line in the message is an initiator token.
	isInitiator := pl[0][0] == TokenInitiator
	if isInitiator != true {
		return errInvalidPattern(errMustBeInitiator)
	}
	prevIsInitiator := !isInitiator

	for _, line := range pl {
		count := map[Token]int{}

		isInitiator = line[0] == TokenInitiator
		// In additional to the rules specified in the noise protocol, it's also
		// required that the initiator/responder cannot send two consecutive
		// messages, they must alternate. For instance,
		//   -> e, s
		//   <- e, ee, se
		// is a legal patter, while,
		//   -> e, s
		//   -> e, ee, se
		// is not legal as they are both from the initiator(->)
		if prevIsInitiator == isInitiator {
			return errInvalidPattern(errConsecutiveTokens, line[0])
		}
		prevIsInitiator = isInitiator

		// TODO: psk token can only be at the begining or end of a line

		for _, token := range line[1:] {
			// check rule 1 and 2 on each pattern line. Not that a "psk" token
			// is allowed to appear one or more times in a handshake pattern.
			if token != TokenPsk && count[token] > 0 {
				return errInvalidPattern(errRepeatedTokens, token)
			}

			count[token]++
			tokenSeen[token]++

			if isInitiator {
				// check rule 3 and 4
				switch token {
				case TokenSe:
					// must have seen an "ee" token before
					if tokenSeen[TokenEe] < 1 {
						return errInvalidPattern(
							errMissingToken, TokenEe, TokenSe)
					}
				case TokenSs:
					// must have seen an "es" token before
					if tokenSeen[TokenEs] < 1 {
						return errInvalidPattern(
							errMissingToken, TokenEs, TokenSs)
					}
				}
			} else {
				// check rule 5 and 6
				switch token {
				case TokenEs:
					// must have seen an "ee" token before
					if tokenSeen[TokenEe] < 1 {
						return errInvalidPattern(
							errMissingToken, TokenEe, TokenEs)
					}
				case TokenSs:
					// must have seen an "se" token before
					if tokenSeen[TokenSe] < 1 {
						return errInvalidPattern(
							errMissingToken, TokenSe, TokenSs)
					}
				}
			}
		}
	}
	return nil
}
