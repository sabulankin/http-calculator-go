package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	_ "github.com/lib/pq"
)

type CalcResponse struct {
	ID     int64    `json:"id,omitempty"`
	Result *float64 `json:"result,omitempty"`
	Error  string   `json:"error,omitempty"`
}

type Calculation struct {
	ID         int64     `json:"id"`
	Expression string    `json:"expression"`
	Result     float64   `json:"result"`
	CreatedAt  time.Time `json:"created_at"`
}

type app struct {
	db *sql.DB
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (a *app) calculateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Используй POST", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
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
	if err != nil {
		writeJSON(w, http.StatusBadRequest, CalcResponse{Error: err.Error()})
		return
	}

	var id int64
	const query = `INSERT INTO calculations (expression, result) VALUES ($1, $2) RETURNING id`
	if err := a.db.QueryRowContext(r.Context(), query, expr, result).Scan(&id); err != nil {
		log.Printf("insert calculation: %v", err)
		writeJSON(w, http.StatusInternalServerError, CalcResponse{Error: "не удалось сохранить результат"})
		return
	}

	writeJSON(w, http.StatusOK, CalcResponse{ID: id, Result: &result})
}

func (a *app) resultsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Используй GET", http.StatusMethodNotAllowed)
		return
	}

	from, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	if err != nil {
		http.Error(w, "Параметр from должен быть в формате RFC3339", http.StatusBadRequest)
		return
	}
	to, err := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "Параметр to должен быть в формате RFC3339", http.StatusBadRequest)
		return
	}
	if from.After(to) {
		http.Error(w, "Параметр from не может быть позже to", http.StatusBadRequest)
		return
	}

	const query = `
		SELECT id, expression, result, created_at
		FROM calculations
		WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at`
	rows, err := a.db.QueryContext(r.Context(), query, from, to)
	if err != nil {
		log.Printf("query calculations: %v", err)
		http.Error(w, "не удалось получить результаты", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	list := make([]Calculation, 0)
	for rows.Next() {
		var calculation Calculation
		if err := rows.Scan(
			&calculation.ID,
			&calculation.Expression,
			&calculation.Result,
			&calculation.CreatedAt,
		); err != nil {
			log.Printf("scan calculation: %v", err)
			http.Error(w, "не удалось прочитать результаты", http.StatusInternalServerError)
			return
		}
		list = append(list, calculation)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate calculations: %v", err)
		http.Error(w, "не удалось прочитать результаты", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, list)
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("db open: ", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("db ping: ", err)
	}

	application := &app{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/calc", application.calculateHandler)
	mux.HandleFunc("/results", application.resultsHandler)
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	server := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Println("Сервер запущен на http://localhost:8081")
	log.Fatal(server.ListenAndServe())
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
				if top == "(" {
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
			return nil, fmt.Errorf("неизвестный токен: %s", tok)
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
