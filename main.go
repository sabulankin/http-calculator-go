package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	_ "github.com/lib/pq"
)

type CalcResponse struct {
	ID      int64   `json:"id,omitempty"`
	Result  float64 `json:"result,omitempty"`
	Error   string  `json:"error,omitempty"`
	Audio   string  `json:"audio,omitempty"`
	Message string  `json:"message,omitempty"`
}
type Calculation struct {
	ID         int64   `json:"id"`
	Expression string  `json:"expression"`
	Result     float64 `json:"result"`
	CreatedAt  string  `json:"created_at"`
}

var db *sql.DB

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

		var id int64
		q := `INSERT INTO calculations (expression, result) VALUES ($1, $2) RETURNING id`
		if err := db.QueryRow(q, expr, result).Scan(&id); err != nil {
			resp.Error = "db insert: " + err.Error()
			resp.Result = 0
		} else {
			resp.ID = id
		}
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		return
	}

}
func resultsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Используй GET", http.StatusMethodNotAllowed)
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		http.Error(
			w,
			"Нужно указать параметры from и to. Пример: /results?from=2026-01-01T00:00:00&to=2030-01-01T00:00:00",
			http.StatusBadRequest,
		)
		return
	}

	rows, err := db.Query(`
		SELECT id, expression, result, created_at
		FROM calculations
		WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at
	`, from, to)
	if err != nil {
		http.Error(w, "db query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []Calculation

	for rows.Next() {
		var c Calculation
		if err := rows.Scan(&c.ID, &c.Expression, &c.Result, &c.CreatedAt); err != nil {
			http.Error(w, "db scan: "+err.Error(), http.StatusInternalServerError)
			return
		}
		list = append(list, c)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func main() {
	dsn := "postgres://postgres:Beton796255@localhost:5433/calculator_db?sslmode=disable"

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("db open:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("db ping:", err)
	}

	http.HandleFunc("/calc", calculateHandler)
	http.HandleFunc("/results", resultsHandler)
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
