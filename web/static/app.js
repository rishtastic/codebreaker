const $ = (id) => document.getElementById(id);
const viewIDs = ["login-view", "lobby-view", "waiting-view", "game-view", "finished-view"];
let state = null;
let events = null;
let eventIdentity = "";
let draftNotebook = null;

function show(id) {
  viewIDs.forEach((viewID) => { $(viewID).hidden = viewID !== id; });
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(body.error || "Something went wrong.");
    error.status = response.status;
    throw error;
  }
  return body;
}

async function refresh() {
  try {
    render(await api("api/state"));
    connect();
  } catch (error) {
    if (error.status === 401) {
      state = null;
      show("login-view");
      setConnection("Locked", false);
      disconnect();
      return;
    }
    setConnection("Reconnecting", false);
  }
}

function render(next) {
	const sameGame = state?.isPlayer && next.isPlayer;
	state = next;
	const identity = next.isPlayer ? `player:${next.seat}` : "visitor";
	if (events && eventIdentity !== identity) disconnect();
  if (!sameGame || !draftNotebook) draftNotebook = structuredClone(next.notebook || emptyNotebook());

  if (next.lobby === "empty") {
    renderLobby("create");
    show("lobby-view");
    return;
  }
  if (next.status === "waiting" && next.isPlayer) {
    show("waiting-view");
    return;
  }
  if (next.lobby === "waiting" && !next.isPlayer) {
    renderLobby("join");
    show("lobby-view");
    return;
  }
  if (!next.isPlayer) {
    renderLobby("full");
    show("lobby-view");
    return;
  }
  if (next.status === "finished") {
    renderFinished(next);
    show("finished-view");
    return;
  }
  renderGame(next);
  show("game-view");
}

function renderLobby(mode) {
  const create = mode === "create";
  const join = mode === "join";
  $("lobby-title").textContent = create ? "Ready to break a code?" : join ? "An opponent is waiting." : "The table is occupied.";
  $("lobby-copy").textContent = create
    ? "Create the only active game, then invite one person to join."
    : join ? "Take the second seat. The game begins as soon as you join." : "Only one game can run at a time.";
  $("name-form").hidden = mode === "full";
  $("create-button").hidden = !create;
  $("join-button").hidden = !join;
  $("full-message").hidden = mode !== "full";
}

function renderGame(next) {
  const me = next.players.find((player) => player.seat === next.seat);
  const opponent = next.players.find((player) => player.seat !== next.seat);
  const finalResponse = next.status === "final_response";
  $("game-title").textContent = finalResponse ? "One final response." : next.canAct ? "Your move." : "Read the table.";
  $("turn-badge").textContent = next.canAct ? (finalResponse ? "Final guess" : `Turn ${next.turn} · your move`) : `Turn ${next.turn} · ${opponent?.name || "opponent"}`;
  $("turn-badge").classList.toggle("active", next.canAct);
  $("final-banner").hidden = !finalResponse;
  $("final-banner").textContent = next.canAct ? "Guess correctly now to force a tie." : "Your opponent has one final guess.";
  $("player-name").textContent = me?.name || "You";
  $("opponent-name").textContent = opponent?.name || "Opponent";
  renderTiles($("own-tiles"), next.ownTiles, false);
  renderTiles($("opponent-tiles"), next.opponentTiles, true);
  renderQuestions(next.available || [], next.canAct && !finalResponse);
  renderHistory(next.history || []);
  renderNotebook();
  $("guess-button").disabled = !next.canAct;
  $("guess-button").textContent = finalResponse ? "Make final guess" : "Guess the code";
  $("resign-button").disabled = next.status !== "active" && next.status !== "final_response";
}

function renderTiles(container, tiles = [], concealed = false) {
  container.textContent = "";
  for (let index = 0; index < 5; index += 1) {
    const tile = tiles[index];
    const element = document.createElement("div");
    element.className = `tile ${concealed && !tile ? "concealed" : tile?.color || ""}`;
    const number = document.createElement("span");
    number.className = "number";
    number.textContent = tile ? tile.number : String.fromCharCode(65 + index);
    const label = document.createElement("span");
    label.className = "color-name";
    label.textContent = tile ? tile.color : "hidden";
    element.append(number, label);
    container.append(element);
  }
}

function renderQuestions(questions, enabled) {
  const container = $("questions");
  container.textContent = "";
  $("question-count").textContent = `${questions.length} card${questions.length === 1 ? "" : "s"}`;
  questions.forEach((question, index) => {
    const card = document.createElement("div");
    card.className = "question";
    const id = document.createElement("span");
    id.className = "q-id";
    id.textContent = `Question ${index + 1}`;
    const prompt = document.createElement("span");
    prompt.className = "q-prompt";
    prompt.textContent = question.prompt.replace("{number}", question.choices?.join(" or ") || "");
    card.append(id, prompt);
    const choices = question.choices || [];
    if (choices.length > 1) {
      const row = document.createElement("div");
      row.className = "question-choices";
      choices.forEach((choice) => {
        const button = document.createElement("button");
        button.type = "button";
        button.textContent = choice;
        button.disabled = !enabled;
        button.setAttribute("aria-label", `Ask about number ${choice}`);
        button.addEventListener("click", () => ask(question.id, choice));
        row.append(button);
      });
      card.append(row);
    } else {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "question-cover";
      button.disabled = !enabled;
      button.setAttribute("aria-label", `Ask: ${question.prompt}`);
      button.addEventListener("click", () => ask(question.id, null));
      card.append(button);
    }
    container.append(card);
  });
}

async function ask(id, choice) {
  try {
    await api("api/game/question", { method: "POST", body: JSON.stringify({ id, choice }) });
    await refresh();
  } catch (error) { announce(error.message); }
}

function renderHistory(history) {
  const container = $("history");
  container.textContent = "";
  [...history].reverse().forEach((item) => {
    const event = document.createElement("div");
    event.className = `event ${item.kind}`;
    event.textContent = item.message;
    container.append(event);
  });
}

function emptyNotebook() {
  return Array.from({ length: 5 }, () => ({ numbers: [], colors: [], text: "" }));
}

function renderNotebook() {
  const container = $("notebook");
  container.textContent = "";
  draftNotebook.forEach((slot, index) => {
    const wrapper = document.createElement("div");
    wrapper.className = "note-slot";
    const title = document.createElement("div");
    title.className = "slot-title";
    title.textContent = String.fromCharCode(65 + index);
    const numbers = document.createElement("div");
    numbers.className = "number-notes";
    for (let value = 0; value <= 9; value += 1) numbers.append(noteCheck(index, "numbers", value, String(value), slot.numbers?.includes(value)));
    const colors = document.createElement("div");
    colors.className = "color-notes";
    ["black", "white", "green"].forEach((value) => colors.append(noteCheck(index, "colors", value, value, slot.colors?.includes(value))));
    const text = document.createElement("textarea");
    text.maxLength = 120;
    text.placeholder = "Your note";
    text.value = slot.text || "";
    text.setAttribute("aria-label", `Note for position ${String.fromCharCode(65 + index)}`);
    text.addEventListener("input", () => { draftNotebook[index].text = text.value; markNotesDirty(); });
    wrapper.append(title, numbers, colors, text);
    container.append(wrapper);
  });
}

function noteCheck(index, key, value, labelText, checked) {
  const label = document.createElement("label");
  const input = document.createElement("input");
  input.type = "checkbox";
  input.checked = Boolean(checked);
  input.addEventListener("change", () => {
    const values = draftNotebook[index][key] || [];
    draftNotebook[index][key] = input.checked ? [...values, value] : values.filter((item) => item !== value);
    markNotesDirty();
  });
  label.append(input, document.createTextNode(labelText));
  return label;
}

function markNotesDirty() { $("notes-status").textContent = "Unsaved changes"; }

async function saveNotes() {
  try {
    await api("api/notebook", { method: "PUT", body: JSON.stringify({ slots: draftNotebook }) });
    $("notes-status").textContent = "Saved";
  } catch (error) { $("notes-status").textContent = error.message; }
}

function openGuess() {
  if (!state?.canAct) return;
  let dialog = $("guess-dialog");
  if (!dialog) {
    dialog = document.createElement("dialog");
    dialog.id = "guess-dialog";
    document.body.append(dialog);
  }
  dialog.textContent = "";
  const form = document.createElement("form");
  form.className = "guess-form";
  const title = document.createElement("h2");
  title.textContent = state.status === "final_response" ? "Your final response" : "Guess the code";
  const copy = document.createElement("p");
  copy.className = "muted";
  copy.textContent = "Enter all five tiles. An incorrect guess reveals nothing.";
  const grid = document.createElement("div");
  grid.className = "guess-grid";
  for (let index = 0; index < 5; index += 1) grid.append(guessField(index));
  const error = document.createElement("p");
  error.className = "error";
  const actions = document.createElement("div");
  actions.className = "button-row";
  const submit = document.createElement("button");
  submit.type = "submit";
  submit.className = "primary";
  submit.textContent = "Submit guess";
  const cancel = document.createElement("button");
  cancel.type = "button";
  cancel.className = "ghost";
  cancel.textContent = "Cancel";
  cancel.addEventListener("click", () => dialog.close());
  actions.append(submit, cancel);
  form.append(title, copy, grid, error, actions);
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(form);
    const tiles = Array.from({ length: 5 }, (_, index) => ({ number: Number(data.get(`number-${index}`)), color: data.get(`color-${index}`) }));
    try {
      await api("api/game/guess", { method: "POST", body: JSON.stringify({ tiles }) });
      dialog.close();
      await refresh();
    } catch (problem) { error.textContent = problem.message; }
  });
  dialog.append(form);
  dialog.showModal();
}

function guessField(index) {
  const label = document.createElement("label");
  label.textContent = String.fromCharCode(65 + index);
  const number = document.createElement("select");
  number.name = `number-${index}`;
  for (let value = 0; value <= 9; value += 1) number.add(new Option(String(value), String(value)));
  const color = document.createElement("select");
  color.name = `color-${index}`;
  setColors(number, color);
  number.addEventListener("change", () => setColors(number, color));
  label.append(number, color);
  return label;
}

function setColors(number, color) {
  color.textContent = "";
  const values = number.value === "5" ? ["green"] : ["black", "white"];
  values.forEach((value) => color.add(new Option(value, value)));
}

function renderFinished(next) {
  const winner = next.winner == null ? null : next.players.find((player) => player.seat === next.winner);
  $("finished-title").textContent = next.tie ? "Both codes were broken." : winner?.seat === next.seat ? "You broke the code." : winner ? `${winner.name} broke the code.` : "No one broke the code.";
  $("finished-copy").textContent = next.tie ? "The game ends in a tie." : winner ? (winner.seat === next.seat ? "You won the game." : "Your opponent won the game.") : "All question cards were used.";
  renderTiles($("revealed-code"), next.opponentTiles, false);
  $("rematch-button").disabled = next.rematchRequested;
  $("rematch-status").textContent = next.rematchRequested ? "Waiting for your opponent to accept the rematch." : "A rematch starts when both players request one.";
}

function connect() {
	if (events) return;
	eventIdentity = state?.isPlayer ? `player:${state.seat}` : "visitor";
	events = new EventSource("api/events");
  events.addEventListener("state", (event) => {
    try { render(JSON.parse(event.data)); } catch { refresh(); }
    setConnection("Live", true);
  });
  events.onopen = () => setConnection("Live", true);
  events.onerror = () => setConnection("Reconnecting", false);
}

function disconnect() { events?.close(); events = null; eventIdentity = ""; }
function setConnection(text, live) { $("connection").textContent = text; $("connection").classList.toggle("live", live); }
function announce(message) { $("connection").textContent = message; setTimeout(() => setConnection("Live", true), 2800); }

$("login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await api("api/auth/login", { method: "POST", body: JSON.stringify({ password: $("password").value }) });
    $("password").value = "";
    $("login-error").textContent = "";
    await refresh();
  } catch (error) { $("login-error").textContent = error.message; }
});

$("name-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await api("api/game/create", { method: "POST", body: JSON.stringify({ name: $("name").value }) });
    await refresh();
  } catch (error) { $("lobby-error").textContent = error.message; }
});

$("join-button").addEventListener("click", async () => {
  try {
    await api("api/game/join", { method: "POST", body: JSON.stringify({ name: $("name").value }) });
    await refresh();
  } catch (error) { $("lobby-error").textContent = error.message; }
});

$("cancel-button").addEventListener("click", async () => { try { await api("api/game/cancel", { method: "POST", body: "{}" }); await refresh(); } catch (error) { announce(error.message); } });
$("guess-button").addEventListener("click", openGuess);
$("save-notes").addEventListener("click", saveNotes);
$("resign-button").addEventListener("click", async () => { if (!confirm("Resign this game?")) return; try { await api("api/game/resign", { method: "POST", body: "{}" }); await refresh(); } catch (error) { announce(error.message); } });
$("rematch-button").addEventListener("click", async () => { try { await api("api/game/rematch", { method: "POST", body: "{}" }); await refresh(); } catch (error) { announce(error.message); } });
$("close-button").addEventListener("click", async () => { try { await api("api/game/close", { method: "POST", body: "{}" }); await refresh(); } catch (error) { announce(error.message); } });

refresh();
setInterval(() => { if (!document.hidden && state) refresh(); }, 15000);
