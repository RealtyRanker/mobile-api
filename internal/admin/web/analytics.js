import { available, dateLabel, el, empty, number, sourceBadge, sourceName } from "./ui.js";

const colors = { cian: "#4b61a9", yandex: "#a44f32", other: "#697571" };

export function renderAnalytics(root, data, days) {
  const cards = el("div", "metric-grid");
  for (const [label, value] of [
    ["Уникальных квартир", data.unique_flats], ["Объявлений", data.listings], ["На нескольких площадках", data.shared_flats],
  ]) {
    const metric = el("section", "panel metric");
    metric.append(el("h2", "", label), el("strong", "", number(value)));
    cards.append(metric);
  }
  const stats = el("section", "panel analytics-table");
  stats.append(el("h2", "section-title", "Сравнение площадок"));
  const statsByPlatform = new Map(data.platforms.map((row) => [row.source, row]));
  const platforms = [...new Set(["cian", "yandex", ...statsByPlatform.keys()])];
  const rows = platforms.map((platform) => {
    const row = statsByPlatform.get(platform);
    return [
      sourceBadge(platform), number(row?.listings ?? 0), number(row?.unique_flats ?? 0),
      available(row?.average_price, " ₽"), available(row?.median_price, " ₽"),
      available(row?.average_area, " м²"), available(row?.average_sq_meter, " ₽"),
      available(row?.average_score), number(row?.shared_flats ?? 0),
      row?.listings ? `${number(100 * row.with_photos / row.listings)}%` : "—",
    ];
  });
  stats.append(table(["Площадка", "Объявлений", "Квартир", "Средняя цена", "Медиана цены", "Средняя площадь",
    "Цена за м²", "Средний скор", "Совпадений", "С фото"], rows));
  stats.append(el("p", "muted analytics-note",
    "Квартира с несколькими ссылками учитывается один раз в общем числе. В строке площадки учитываются её объявления и цены."));

  const chart = el("section", "panel chart-panel");
  const header = el("div", "chart-heading");
  const heading = el("div");
  heading.append(el("h2", "section-title", "Объявления по дням"), el("p", "muted",
    days === 0 ? "На графике последние 90 дней. В таблице доступен весь период." : "По дате первого сбора квартиры, московское время."));
  const legend = el("div", "chart-legend");
  for (const platform of platforms) {
    const label = el("span");
    label.append(el("i", `swatch source-${platform}`), document.createTextNode(sourceName(platform)));
    legend.append(label);
  }
  header.append(heading, legend);
  chart.append(header);
  let cleanup;
  if (data.volume.length) {
    const canvas = el("canvas", "chart");
    canvas.setAttribute("role", "img");
    canvas.setAttribute("aria-label", "График числа объявлений по дням и площадкам. Значения доступны в таблице ниже.");
    chart.append(canvas);
    const details = el("details");
    const summary = el("summary", "", "Таблица по дням");
    const daysWithListings = [...new Set(data.volume.map((point) => point.day))];
    const counts = byDay(data.volume);
    details.append(summary, table(["Дата", ...platforms.map(sourceName)],
      daysWithListings.map((day) => [dateLabel(day), ...platforms.map((platform) => number(counts.get(day)?.get(platform) ?? 0))])));
    chart.append(details);
    cleanup = () => observer.disconnect();
    const date = new Intl.DateTimeFormat("sv-SE", { timeZone: "Europe/Moscow" }).format(new Date());
    const end = new Date(`${date}T00:00:00Z`);
    const range = days || 90;
    const bars = Array.from({ length: range }, (_, index) => {
      const day = new Date(end.valueOf() - (range - index - 1) * 86400000).toISOString().slice(0, 10);
      return { day, counts: platforms.map((platform) => counts.get(day)?.get(platform) ?? 0) };
    });
    const observer = new ResizeObserver(() => draw(canvas, bars, platforms));
    observer.observe(canvas);
  } else {
    chart.append(empty("За выбранный период нет данных", "Попробуйте изменить период, регион или тип сделки."));
  }
  root.replaceChildren(cards, chart, stats);
  return cleanup;
}

function byDay(volume) {
  const result = new Map();
  for (const { day, source, listings } of volume) {
    if (!result.has(day)) result.set(day, new Map());
    result.get(day).set(source, listings);
  }
  return result;
}

function table(columns, rows) {
  const scroll = el("div", "table-scroll");
  const item = el("table");
  const head = item.createTHead().insertRow();
  for (const column of columns) {
    const cell = el("th", "", column);
    cell.scope = "col";
    head.append(cell);
  }
  const body = item.createTBody();
  for (const [first, ...rest] of rows) {
    const row = body.insertRow();
    for (const value of [first, ...rest]) {
      const cell = row.insertCell();
      if (value instanceof Node) cell.append(value);
      else cell.textContent = value;
    }
  }
  scroll.append(item);
  return scroll;
}

function draw(canvas, bars, platforms) {
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  if (!width || !height) return;
  const ratio = window.devicePixelRatio || 1;
  canvas.width = Math.floor(width * ratio);
  canvas.height = Math.floor(height * ratio);
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.scale(ratio, ratio);
  const left = 36, top = 10, right = 7, bottom = 29;
  const plotWidth = width - left - right;
  const plotHeight = height - top - bottom;
  const max = Math.max(1, ...bars.map((bar) => bar.counts.reduce((a, b) => a + b, 0)));
  const step = Math.ceil(max / 4);
  const maxTick = Math.ceil(max / step) * step;
  ctx.font = "11px system-ui";
  ctx.textBaseline = "middle";
  ctx.textAlign = "right";
  for (let value = 0; value <= maxTick; value += step) {
    const y = top + (1 - value / maxTick) * plotHeight;
    ctx.strokeStyle = "#dfe7df";
    ctx.beginPath();
    ctx.moveTo(left, y);
    ctx.lineTo(width - right, y);
    ctx.stroke();
    ctx.fillStyle = "#637168";
    ctx.fillText(number(value), left - 5, y);
  }
  const slot = plotWidth / bars.length;
  const barWidth = Math.max(1, Math.min(34, slot * .72));
  const every = Math.max(1, Math.ceil(bars.length / Math.max(2, Math.floor(plotWidth / 66))));
  ctx.textAlign = "center";
  bars.forEach((bar, index) => {
    const x = left + (index + .5) * slot;
    let y = top + plotHeight;
    bar.counts.forEach((count, i) => {
      const size = (count / maxTick) * plotHeight;
      y -= size;
      ctx.fillStyle = colors[platforms[i]] || colors.other;
      ctx.fillRect(x - barWidth / 2, y, barWidth, size);
    });
    if (index % every === 0 && x >= left + 12) {
      ctx.fillStyle = "#637168";
      ctx.fillText(`${bar.day.slice(8, 10)}.${bar.day.slice(5, 7)}`, x, height - 12);
    }
  });
}
