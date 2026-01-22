package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type CalcResponse struct {
	Result  float64 `json:"result,omitempty"`
	Error   string  `json:"error,omitempty"`
	Audio   string  `json:"audio,omitempty"`
	Message string  `json:"message,omitempty"`
}

func calculateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Используй POST", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Не удалось прочитать тело запроса", http.StatusBadRequest)
		return
	}

	expr := strings.TrimSpace(string(body))
	if expr == "" {
		http.Error(w, "Пустое выражение", http.StatusBadRequest)
		return
	}

	result, err := eval(expr)

	resp := CalcResponse{}
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Result = result
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		return
	}

}

func main() {
	http.HandleFunc("/calc", calculateHandler)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	fmt.Println("Серевер запущен на http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func eval(expr string) (float64, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return 0, err
	}

	rpn, err := shuntingYard(tokens)
	if err != nil {
		return 0, err
	}

	return evalRPN(rpn)
}

func tokenize(expr string) ([]string, error) {
	var tokens []string
	var number strings.Builder
	prev := ""

	for i, ch := range expr {
		if !unicode.IsDigit(ch) && ch != '.' && ch != '+' && ch != '-' && ch != '*' && ch != '/' && ch != '(' && ch != ')' && !unicode.IsSpace(ch) {
			return nil, fmt.Errorf("недопустимый символ: %q", ch)
		}

		if unicode.IsDigit(ch) || ch == '.' || ch == '-' && (i == 0) {
		}

		if unicode.IsDigit(ch) || ch == '.' {
			number.WriteRune(ch)
		} else {
			if number.Len() > 0 {
				tokens = append(tokens, number.String())
				number.Reset()
				prev = tokens[len(tokens)-1]
			}
			if unicode.IsSpace(ch) {
				continue
			}

			if ch == '-' {
				if i == 0 || prev == "" || prev == "(" || prev == "+" || prev == "-" || prev == "*" || prev == "/" {
					number.WriteRune(ch)
					continue
				}
			}

			tokens = append(tokens, string(ch))
			prev = string(ch)
		}
	}

	if number.Len() > 0 {
		tokens = append(tokens, number.String())
	}

	return tokens, nil
}

func shuntingYard(tokens []string) ([]string, error) {
	var output []string
	var stack []string
	prec := map[string]int{"+": 1, "-": 1, "*": 2, "/": 2}

	for _, tok := range tokens {
		if isNumber(tok) {
			output = append(output, tok)
		} else if tok == "+" || tok == "-" || tok == "*" || tok == "/" {
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				if top == "()" {
					break
				}
				if prec[top] >= prec[tok] {
					output = append(output, top)
					stack = stack[:len(stack)-1]
				} else {
					break
				}
			}
			stack = append(stack, tok)
		} else if tok == "(" {
			stack = append(stack, tok)
		} else if tok == ")" {
			found := false
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if top == "(" {
					found = true
					break
				}
				output = append(output, top)
			}
			if !found {
				return nil, fmt.Errorf("несоответствие скобок")
			}
		} else {
			return nil, fmt.Errorf("неизвестный токе: %s", tok)
		}
	}

	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if top == "(" || top == ")" {
			return nil, fmt.Errorf("несоответствие скобок")
		}
		output = append(output, top)
	}
	return output, nil
}

func evalRPN(tokens []string) (float64, error) {
	var stack []float64
	for _, tok := range tokens {
		if isNumber(tok) {
			num, err := strconv.ParseFloat(tok, 64)
			if err != nil {
				return 0, err
			}
			stack = append(stack, num)
		} else {
			if len(stack) < 2 {
				return 0, fmt.Errorf("недостаточно операндов")
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			switch tok {
			case "+":
				stack = append(stack, a+b)
			case "-":
				stack = append(stack, a-b)
			case "*":
				stack = append(stack, a*b)
			case "/":
				if b == 0 {
					return 0, fmt.Errorf("деление на ноль")
				}
				stack = append(stack, a/b)
			default:
				return 0, fmt.Errorf("неизвестный оператор: %s", tok)
			}
		}
	}
	if len(stack) != 1 {
		return 0, fmt.Errorf("ошибка вычислений")
	}
	return stack[0], nil
}
func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
