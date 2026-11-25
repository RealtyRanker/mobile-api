import { $, available, dateLabel, el, flatTitle, number, photo, price, safeURL, sourceBadge } from "./ui.js";

export function renderDetail(root, flat, regionName) {
  const heading = el("header", "detail-heading");
  heading.append(el("h1", "", flatTitle(flat.room_number, flat.total_area)));
  heading.append(el("p", "", [flat.address || regionName, ...(flat.underground_stations || [])].filter(Boolean).join(" · ")));
  const layout = el("div", "detail-layout");
  layout.append(gallery(flat.listings), sources(flat), facts(flat));
  root.replaceChildren(heading, layout);
}

function collectPhotos(listings) {
  const result = new Map();
  for (const listing of listings) {
    const originals = listing.photo_urls || [];
    const thumbs = listing.photo_thumbnail_urls || [];
    for (let i = 0; i < Math.max(originals.length, thumbs.length); i++) {
      const url = safeURL(originals[i]) || safeURL(thumbs[i]);
      if (!url || result.has(url)) continue;
      result.set(url, { url, thumb: safeURL(thumbs[i]) || url });
    }
  }
  return [...result.values()];
}

function gallery(listings) {
  const pictures = collectPhotos(listings);
  const panel = el("section", "panel detail-gallery");
  panel.append(el("h2", "section-title", pictures.length ? `Фотографии · ${number(pictures.length)}` : "Фотографии"));
  if (pictures.length === 0) {
    panel.append(el("div", "photo-placeholder", "В объявлении нет фотографий"));
    return panel;
  }
  let selected = 0;
  const main = el("button", "gallery-hero");
  main.type = "button";
  main.setAttribute("aria-label", "Увеличить фотографию");
  const thumbs = el("div", "gallery-thumbs");
  const thumbButtons = pictures.map(({ url, thumb }, index) => {
    const button = el("button", "thumb");
    button.type = "button";
    button.setAttribute("aria-label", `Показать фото ${index + 1}`);
    button.append(photo(thumb, "", true, url));
    button.addEventListener("click", () => select(index));
    thumbs.append(button);
    return button;
  });
  function select(index) {
    selected = (index + pictures.length) % pictures.length;
    const { url, thumb } = pictures[selected];
    main.replaceChildren(photo(url, `Фото квартиры ${selected + 1}`, false, thumb),
      el("span", "photo-number", `${selected + 1} / ${pictures.length}`));
    thumbButtons.forEach((item, index) => item.setAttribute("aria-pressed", String(index === selected)));
  }
  main.addEventListener("click", () => openPhoto(pictures, selected, select));
  select(0);
  panel.append(main, thumbs);
  return panel;
}

function openPhoto(pictures, start, select) {
  const dialog = $("#photo-dialog");
  let index = start;
  const tools = el("div", "dialog-tools");
  const controls = el("div", "range");
  const prev = el("button", "secondary", "←");
  const next = el("button", "secondary", "→");
  const close = el("button", "secondary", "Закрыть");
  prev.setAttribute("aria-label", "Предыдущее фото");
  next.setAttribute("aria-label", "Следующее фото");
  const counter = el("span", "");
  controls.append(prev, counter, next);
  tools.append(controls, close);
  const image = el("div");
  dialog.replaceChildren(tools, image);
  function show(newIndex) {
    index = (newIndex + pictures.length) % pictures.length;
    counter.textContent = `${index + 1} / ${pictures.length}`;
    image.replaceChildren(photo(pictures[index].url, `Фото квартиры ${index + 1}`, false, pictures[index].thumb));
    select(index);
  }
  prev.disabled = next.disabled = pictures.length < 2;
  prev.onclick = () => show(index - 1);
  next.onclick = () => show(index + 1);
  close.onclick = () => dialog.close();
  dialog.onclick = (event) => { if (event.target === dialog) dialog.close(); };
  dialog.onkeydown = (event) => {
    if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
      event.preventDefault();
      show(index + (event.key === "ArrowLeft" ? -1 : 1));
    }
  };
  show(start);
  dialog.showModal();
  close.focus();
}

function sources(flat) {
  const panel = el("section", "panel detail-side");
  const info = el("div");
  const amount = el("div", "detail-price");
  amount.append(price(flat.price, flat.deal_type));
  const dates = el("div", "detail-dates");
  dates.append(el("div", "", `Обновлено ${dateLabel(flat.updated_at, true)}`),
    el("div", "", `Собрано ${dateLabel(flat.parsed_at, true)}`));
  info.append(amount, dates);
  const platforms = el("div");
  platforms.append(el("h2", "section-title", "Площадки"));
  const list = el("ul", "platform-list");
  for (const listing of flat.listings) {
    const item = el("li");
    const url = safeURL(listing.link);
    if (url) {
      const link = el("a");
      link.href = url;
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.append(sourceBadge(listing.source), el("span", "", "Открыть ↗"));
      item.append(link);
    } else {
      item.append(sourceBadge(listing.source));
    }
    list.append(item);
  }
  platforms.append(list);
  panel.append(info, platforms);
  return panel;
}

function facts(f) {
  const panel = el("section", "panel detail-facts");
  panel.append(el("h2", "section-title", "Характеристики"));
  const groups = el("div", "fact-groups");
  const yes = (value) => value ? "Да" : "";
  const floor = f.floor > 0 ? f.max_floor > 0 ? `${f.floor} из ${f.max_floor}` : f.floor : "";
  const renovation = { cosmetic: "Косметический", euro: "Евроремонт", design: "Дизайнерский", renovated: "Есть ремонт", none: "Без ремонта", noRepair: "Без ремонта" };
  const view = { yard: "Во двор", street: "На улицу", both: "Во двор и на улицу" };
  const parameters = [
    ["Квартира", [
      ["Общая площадь", available(f.total_area, " м²")], ["Жилая площадь", available(f.living_area, " м²")],
      ["Площадь кухни", available(f.kitchen_area, " м²")], ["Этаж", floor], ["Потолки", available(f.ceiling_height, " м")],
      ["Ремонт", renovation[f.renovation] || f.renovation], ["Окна", view[f.windows_view] || f.windows_view],
      ["Раздельные санузлы", f.separated_bathroom_count], ["Совмещённые санузлы", f.combined_bathroom_count],
      ["Балконы", f.balcony_count], ["Лоджии", f.loggia_count], ["Апартаменты", yes(f.is_apartments)],
    ]],
    ["Удобства и дом", [
      ["Посудомоечная машина", yes(f.has_dishwasher)], ["Кондиционер", yes(f.has_conditioner)],
      ["Подъезды", f.building_entrances_number], ["Квартир в доме", f.building_apartments_number],
      ["Лифты", f.building_elevators_number], ["Программа реновации", yes(f.demolished_in_moscow_program)],
      ["Координаты", f.latitude && f.longitude ? `${f.latitude.toFixed(5)}, ${f.longitude.toFixed(5)}` : ""],
    ]],
    ["Расположение и скор", [
      ["Скор", available(f.score)], ["Место станции", f.underground_place > 0 && f.underground_place < 1001 ? f.underground_place : ""],
      ["До метро", f.underground_distance_info?.replace(/,\s*$/, "") || f.underground_stations?.join(", ")],
    ]],
    [f.deal_type === "rent" ? "Аренда" : "Продажа", f.deal_type === "rent" ? [
      ["Залог", available(f.deposit, " ₽")], ["Предоплата", available(f.deposit_months, " мес.")],
      ["Комиссия", available(f.comission, "%")], ["Можно с детьми", yes(f.children_allowed)],
      ["Можно с животными", yes(f.pets_allowed)],
    ] : [
      ["Тип продажи", { free: "Свободная", alternative: "Альтернативная" }[f.sale_type] || f.sale_type],
      ["Комиссия", available(f.comission, "%")], ["Ипотека", yes(f.mortgage_allowed)], ["Новостройка", yes(f.is_new_building)],
      ["ЖК", f.new_building_name], ["От собственника", yes(f.is_by_homeowner)],
    ]],
  ];
  for (const [title, entries] of parameters) {
    const present = entries.filter(([, value]) => value && value !== "—");
    if (!present.length) continue;
    const group = el("section", "fact-group");
    const list = el("dl");
    for (const [label, value] of present) {
      const line = el("div");
      line.append(el("dt", "", label), el("dd", "", value));
      list.append(line);
    }
    group.append(el("h3", "", title), list);
    groups.append(group);
  }
  if (!groups.childElementCount) groups.append(el("p", "muted", "Дополнительные характеристики отсутствуют."));
  panel.append(groups);
  return panel;
}
