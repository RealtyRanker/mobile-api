export const $ = (selector, root = document) => root.querySelector(selector);

export function el(tag, className, text) {
  const item = document.createElement(tag);
  if (className) item.className = className;
  if (text !== undefined && text !== null) item.textContent = text;
  return item;
}

const numbers = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 });
const dates = new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeZone: "Europe/Moscow" });
const times = new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short", timeZone: "Europe/Moscow" });
const sources = { cian: "Циан", yandex: "Яндекс Недвижимость", other: "Другие" };

export const sourceName = (source) => sources[source] ?? sources.other;
export const number = (value) => numbers.format(value);
export const available = (value, suffix = "") => value > 0 ? `${number(value)}${suffix}` : "—";
export const dateLabel = (value, withTime = false) => {
  const date = new Date(value);
  if (!value || Number.isNaN(date.valueOf()) || date.getUTCFullYear() <= 1970) return "—";
  return (withTime ? times : dates).format(date);
};
export const flatTitle = (rooms, area) => {
  const type = rooms === 0 ? "Студия" : `${rooms}-комнатная`;
  return area > 0 ? `${type} · ${number(area)} м²` : type;
};

export function sourceBadge(source) {
  const badge = el("span", `source-badge source-${source}`);
  badge.append(el("i", "source-dot"), document.createTextNode(sourceName(source)));
  return badge;
}

export function price(value, deal) {
  const result = el("span", "", available(value, " ₽"));
  if (value > 0 && deal === "rent") result.append(el("small", "price-unit", "/ мес."));
  return result;
}

export function safeURL(value) {
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password ? url.href : null;
  } catch {
    return null;
  }
}

export function photo(url, alt, lazy = true, fallback) {
  const href = safeURL(url);
  if (!href) return el("div", "photo-placeholder", "Нет фото");
  const image = el("img");
  image.alt = alt;
  image.decoding = "async";
  image.loading = lazy ? "lazy" : "eager";
  image.referrerPolicy = "no-referrer";
  image.addEventListener("error", () => {
    if (fallback && href !== safeURL(fallback)) {
      image.replaceWith(photo(fallback, alt, lazy));
    } else {
      image.replaceWith(el("div", "photo-placeholder", "Фото недоступно"));
    }
  }, { once: true });
  image.src = href;
  return image;
}

export function empty(title, subtitle) {
  const state = el("div", "empty");
  state.append(el("h3", "", title));
  if (subtitle) state.append(el("p", "", subtitle));
  return state;
}

export async function readJSON(path, signal) {
  const response = await fetch(path, { signal, headers: { Accept: "application/json" } });
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || "Ошибка загрузки");
  return body;
}
