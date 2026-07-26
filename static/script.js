document.getElementById("calcBtn").addEventListener("click", async () => {
  const expr = document.getElementById("expr").value.trim();
  if (!expr) {
    alert("Введите выражение");
    return;
  }

  const result = document.getElementById("result");
  result.textContent = "Загрузка...";

  try {
    const response = await fetch("/calc", {
      method: "POST",
      headers: {"Content-Type": "text/plain; charset=utf-8"},
      body: expr
    });
    const data = await response.json();
    if (!response.ok || data.error) {
      result.textContent = "Ошибка: " + (data.error || response.statusText);
      return;
    }
    result.textContent = "Результат: " + data.result;
  } catch (error) {
    result.textContent = "Сетевая ошибка: " + error;
  }
});
