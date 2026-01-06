document.getElementById("calcBth").addEventListener("click", async() => {
  const expr = documents.getElementById("expr"). value.trim();
  if (!expr) return alert("Введите выражение");

  const resDiv = document.getElementById("result");
  resDiv.textContent = "Загрузка...";

  try{
    const resp = await fetch("/calc",{
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({expr})
    });
    const data = await resp.json();
    if (data.error) {
      resDiv.textContent = "Ошибка" + data.error;
      return;
    }
    resDiv.textContent = "Результат: " + data.result + (data.message ? ("-" + data.message) : "");

    if (data.audio){
      const audio = document.getElementById("audio");
      audio.src = data.audio;
      try {
        await audio.play();
      } catch (err){
        console.warn("Автовопроизведение может быть запрещено браузером - пользователь должен нажать", err);
      }
    }
  } catch (err) {
    resDiv.textContent = "Сетевая ошибка: " + err;
  }
})