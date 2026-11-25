import { $, dateLabel, el, empty, flatTitle, number, photo, price, readJSON, sourceBadge } from "./ui.js";
import { renderDetail } from "./detail.js";
import { renderAnalytics } from "./analytics.js";

const catalogDefault = "#/flats?deal_type=rent";
const form = $("#filters");
const analyticsForm = $("#analytics-filters");
const region = form.elements.region;
const locationType = $("#location-type");
const locationItems = $("#location-items");
const locationPicker = $("#location-picker");
const locationSearch = $("#location-search");
const sort = $("#sort");
let options;
let optionsRequest;
let request;
let cleanupChart;
let lastCatalog = catalogDefault;
const advancedKeys = ["min_score", "min_metro_rank", "min_kitchen", "min_ceiling", "min_floor", "max_floor",
  "renovation", "bathroom", "children", "pets", "dishwasher", "conditioner", "balcony"];

function routeHash(path, params) {
  return `#${path}${params.size ? `?${params}` : ""}`;
}

function navigate(path, params) {
  const target = routeHash(path, params);
  if (location.hash === target) showPage();
  else location.hash = target;
}

function parameters(form) {
  const result = new URLSearchParams();
  for (const [key, value] of new FormData(form)) {
    if (value !== "") result.append(key, value);
  }
  return result;
}

function setForm(form, query) {
  for (const field of form.elements) {
    if (!field.name || field.name === "location") continue;
    const selected = query.getAll(field.name);
    if (field.type === "checkbox") field.checked = selected.includes(field.value);
    else field.value = selected[0] ?? "";
  }
}

function updateDeal() {
  const deal = form.elements.deal_type.value;
  $("#price-label").textContent = deal === "rent" ? "Цена, ₽/мес." : "Цена, ₽";
  for (const label of form.querySelectorAll(".rent-only")) {
    label.hidden = deal === "sale";
    $("input", label).disabled = label.hidden;
  }
}

function renderLocations(selected = []) {
  const isMoscow = region.value === "1";
  $("#location-mode").hidden = !isMoscow;
  locationType.disabled = !isMoscow;
  locationPicker.hidden = !isMoscow || !locationType.value;
  locationSearch.value = "";
  locationItems.replaceChildren();
  $("#location-empty").hidden = true;
  if (locationPicker.hidden) return;
  for (const choice of options?.locations[locationType.value] || []) {
    const label = el("label");
    const box = el("input");
    box.type = "checkbox";
    box.name = "location";
    box.value = choice.value;
    box.checked = selected.includes(choice.value);
    label.append(box, document.createTextNode(choice.label));
    locationItems.append(label);
  }
  const selectionCount = locationItems.querySelectorAll("input:checked").length;
  $("#location-label").textContent = selectionCount ? `Выбрано: ${selectionCount}` : "Выбрать из списка";
  locationPicker.open = selectionCount === 0;
}

function regionName(id) {
  return options?.regions.find((item) => item.id === id)?.name || `Регион ${id}`;
}

function initializeOptions() {
  if (!optionsRequest) {
    optionsRequest = readJSON("/api/options").then((data) => {
      options = data;
      for (const select of document.querySelectorAll(".region-select")) {
        select.replaceChildren(new Option("Все регионы", ""),
          ...options.regions.map((item) => new Option(item.name, String(item.id))));
      }
    }).catch((error) => {
      optionsRequest = null;
      throw error;
    });
  }
  return optionsRequest;
}

function setActive(page) {
  for (const id of ["catalog", "detail", "analytics"]) {
    document.getElementById(id).hidden = id !== page;
  }
  for (const [id, active] of [["nav-flats", page !== "analytics"], ["nav-analytics", page === "analytics"]]) {
    const link = document.getElementById(id);
    if (active) link.setAttribute("aria-current", "page");
    else link.removeAttribute("aria-current");
  }
  $("#breadcrumb").textContent = { catalog: "Каталог", detail: "Каталог / Квартира", analytics: "Аналитика" }[page];
  document.title = `${{ catalog: "Квартиры", detail: "Квартира", analytics: "Аналитика" }[page]} · Realty Ranker`;
}

async function showPage() {
  request?.abort();
  cleanupChart?.();
  cleanupChart = null;
  const dialog = $("#photo-dialog");
  if (dialog.open) dialog.close();
  const controller = new AbortController();
  request = controller;
  const signal = controller.signal;
  const [path, search] = (location.hash || catalogDefault).slice(1).split("?");
  const query = new URLSearchParams(search);
  let target;
  if (path === "/analytics") {
    setActive("analytics");
    query.set("deal_type", query.get("deal_type") || "rent");
    query.set("days", query.get("days") ?? "30");
    $("#nav-analytics").href = routeHash(path, query);
    target = $("#analytics-body");
  } else if (/^\/flats\/\d+$/.test(path)) {
    setActive("detail");
    $("#back-to-catalog").href = lastCatalog;
    target = $("#detail-body");
  } else {
    setActive("catalog");
    lastCatalog = routeHash("/flats", query);
    $("#nav-flats").href = lastCatalog;
    target = $("#catalog-status");
    $("#cards").replaceChildren();
    $("#pagination").replaceChildren();
    $("#result-title").textContent = "Объявления";
  }
  target.replaceChildren(el("p", "muted loading", "Загружаем данные…"));
  try {
    await initializeOptions();
    if (signal.aborted) return;
    if (path === "/analytics") {
      setForm(analyticsForm, query);
      const data = await readJSON(`/api/analytics?${query}`, signal);
      if (!signal.aborted) cleanupChart = renderAnalytics(target, data, Number(query.get("days")));
    } else if (target.id === "detail-body") {
      const flat = await readJSON(`/api${path}`, signal);
      if (!signal.aborted) renderDetail(target, flat, regionName(flat.region));
    } else {
      setForm(form, query);
      updateDeal();
      sort.value = query.get("sort") || "newest";
      renderLocations(query.getAll("location"));
      if (advancedKeys.some((key) => query.has(key))) $("#advanced").open = true;
      const data = await readJSON(`/api/flats?${query}`, signal);
      if (!signal.aborted) renderCatalog(data, query);
    }
  } catch (error) {
    if (!signal.aborted) target.replaceChildren(empty("Не удалось загрузить данные", error.message));
  }
}

function renderCatalog(data, query) {
  const heading = $("#result-title");
  heading.replaceChildren(document.createTextNode("Объявления"), el("span", "result-count", number(data.total)));
  const status = $("#catalog-status");
  if (!data.total) {
    status.replaceChildren(empty("Подходящих квартир нет", "Измените значения фильтров или сбросьте их."));
    return;
  }
  status.replaceChildren();
  $("#cards").replaceChildren(...data.flats.map((flat) => listingCard(flat)));
  const count = Math.ceil(data.total / data.page_size);
  if (count <= 1) return;
  const pages = $("#pagination");
  const previous = pageLink("← Назад", data.page - 1, query);
  const next = pageLink("Далее →", data.page + 1, query);
  pages.append(data.page > 1 ? previous : el("button", "secondary", "← Назад"),
    el("span", "", `${data.page} из ${count}`),
    data.page < count ? next : el("button", "secondary", "Далее →"));
  for (const button of pages.querySelectorAll("button")) button.disabled = true;
}

function pageLink(label, page, query) {
  const link = el("a", "secondary", label);
  const params = new URLSearchParams(query);
  if (page > 1) params.set("page", String(page));
  else params.delete("page");
  link.href = routeHash("/flats", params);
  return link;
}

function listingCard(flat) {
  const card = el("a", "listing-card");
  card.href = `#/flats/${flat.id}`;
  const cover = el("div", "card-photo");
  const platforms = el("div", "card-platforms");
  platforms.append(...[flat.source, ...flat.other_sources].map(sourceBadge));
  cover.append(photo(flat.photo, ""), platforms);
  const body = el("div", "card-body");
  const amount = el("div", "card-price");
  amount.append(price(flat.price, flat.deal_type));
  const details = el("div", "card-meta", flat.address || regionName(flat.region));
  const stations = flat.stations || [];
  body.append(amount, el("h3", "card-title", flatTitle(flat.rooms, flat.area)), details);
  if (stations.length) {
    const metro = el("div", "card-meta card-metro", stations.join(" · "));
    metro.title = stations.join(", ");
    body.append(metro);
  }
  const footer = el("div", "card-footer");
  if (flat.score !== 0) footer.append(el("span", "score", `Скор ${number(flat.score)}`));
  footer.append(el("time", "", dateLabel(flat.updated_at)));
  card.append(cover, body, footer);
  return card;
}

function applyFilters(event) {
  event?.preventDefault();
  const params = parameters(form);
  if (sort.value !== "newest") params.set("sort", sort.value);
  if (!params.has("location")) params.delete("location_type");
  navigate("/flats", params);
}

form.addEventListener("submit", applyFilters);
sort.addEventListener("change", () => form.requestSubmit());
$("#reset-filters").addEventListener("click", () => {
  $("#advanced").open = false;
  if (location.hash === catalogDefault) showPage();
  else location.hash = catalogDefault;
});
form.elements.deal_type.addEventListener("change", updateDeal);
region.addEventListener("change", () => {
  if (region.value !== "1") locationType.value = "";
  renderLocations();
});
locationType.addEventListener("change", () => renderLocations());
locationItems.addEventListener("change", () => {
  const count = locationItems.querySelectorAll("input:checked").length;
  $("#location-label").textContent = count ? `Выбрано: ${count}` : "Выбрать из списка";
});
locationSearch.addEventListener("input", () => {
  const search = locationSearch.value.trim().toLocaleLowerCase("ru");
  let visible = 0;
  for (const label of locationItems.children) {
    label.hidden = !label.textContent.toLocaleLowerCase("ru").includes(search);
    if (!label.hidden) visible++;
  }
  $("#location-empty").hidden = visible > 0;
});
const applyAnalytics = (event) => {
  event.preventDefault();
  navigate("/analytics", parameters(analyticsForm));
};
analyticsForm.addEventListener("submit", applyAnalytics);
analyticsForm.addEventListener("change", applyAnalytics);
window.addEventListener("hashchange", () => {
  window.scrollTo(0, 0);
  showPage();
});
window.addEventListener("pageshow", (event) => { if (event.persisted) showPage(); });
showPage();
