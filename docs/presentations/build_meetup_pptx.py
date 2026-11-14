#!/usr/bin/env python3
"""Build GeoAtlas meetup PowerPoint (product/value focus, ~15 min + Q&A)."""

from pathlib import Path

from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import PP_ALIGN
from pptx.util import Inches, Pt

# Visual system: cool ink + teal (avoid purple / cream-terracotta / newspaper)
INK = RGBColor(0x14, 0x21, 0x2B)
INK_SOFT = RGBColor(0x3A, 0x4A, 0x55)
MUTED = RGBColor(0x5C, 0x6B, 0x75)
TEAL = RGBColor(0x0E, 0x7C, 0x66)
TEAL_DARK = RGBColor(0x0A, 0x5C, 0x4C)
SURFACE = RGBColor(0xF4, 0xF8, 0xF7)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
LINE = RGBColor(0xD5, 0xE0, 0xDC)
ACCENT_WARM = RGBColor(0xC4, 0x5C, 0x26)  # sparingly for "problem" emphasis

SLIDE_W = Inches(13.333)
SLIDE_H = Inches(7.5)
MARGIN = Inches(0.7)


def set_run(run, text, *, size=18, bold=False, color=INK, font="Calibri"):
    run.text = text
    run.font.size = Pt(size)
    run.font.bold = bold
    run.font.color.rgb = color
    run.font.name = font


def add_textbox(slide, left, top, width, height):
    return slide.shapes.add_textbox(left, top, width, height)


def fill_shape(shape, color):
    shape.fill.solid()
    shape.fill.fore_color.rgb = color
    shape.line.fill.background()


def add_bg(slide, color=WHITE):
    shape = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, SLIDE_W, SLIDE_H)
    fill_shape(shape, color)
    # send to back
    spTree = slide.shapes._spTree
    sp = shape._element
    spTree.remove(sp)
    spTree.insert(2, sp)
    return shape


def add_top_bar(slide):
    bar = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, SLIDE_W, Inches(0.08))
    fill_shape(bar, TEAL)


def add_footer(slide, page: int, total: int):
    tb = add_textbox(slide, MARGIN, Inches(7.05), Inches(8), Inches(0.35))
    set_run(tb.text_frame.paragraphs[0].add_run(), "ГеоАтлас  ·  meetup", size=11, color=MUTED)
    tb2 = add_textbox(slide, Inches(11.2), Inches(7.05), Inches(1.5), Inches(0.35))
    p2 = tb2.text_frame.paragraphs[0]
    p2.alignment = PP_ALIGN.RIGHT
    set_run(p2.add_run(), f"{page} / {total}", size=11, color=MUTED)


def title_block(slide, title: str, subtitle: str | None = None):
    add_top_bar(slide)
    tb = add_textbox(slide, MARGIN, Inches(0.35), Inches(12), Inches(0.7))
    tf = tb.text_frame
    p = tf.paragraphs[0]
    set_run(p.add_run(), title, size=32, bold=True, color=INK)
    if subtitle:
        tb2 = add_textbox(slide, MARGIN, Inches(0.95), Inches(12), Inches(0.45))
        p2 = tb2.text_frame.paragraphs[0]
        set_run(p2.add_run(), subtitle, size=16, color=MUTED)


def bullet_block(slide, left, top, width, height, items, *, size=18, color=INK):
    tb = add_textbox(slide, left, top, width, height)
    tf = tb.text_frame
    tf.word_wrap = True
    for i, item in enumerate(items):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.level = 0
        p.space_after = Pt(10)
        set_run(p.add_run(), f"•  {item}", size=size, color=color)
    return tb


def card(slide, left, top, width, height, title, body_lines, *, accent=TEAL):
    shape = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, left, top, width, height)
    fill_shape(shape, SURFACE)
    shape.line.color.rgb = LINE
    shape.adjustments[0] = 0.08
    accent_bar = slide.shapes.add_shape(
        MSO_SHAPE.RECTANGLE, left, top, Inches(0.08), height
    )
    fill_shape(accent_bar, accent)
    tb = add_textbox(slide, left + Inches(0.28), top + Inches(0.18), width - Inches(0.4), Inches(0.4))
    set_run(tb.text_frame.paragraphs[0].add_run(), title, size=16, bold=True, color=TEAL_DARK)
    tb2 = add_textbox(
        slide,
        left + Inches(0.28),
        top + Inches(0.55),
        width - Inches(0.4),
        height - Inches(0.7),
    )
    tf = tb2.text_frame
    tf.word_wrap = True
    for i, line in enumerate(body_lines):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.space_after = Pt(6)
        set_run(p.add_run(), line, size=14, color=INK_SOFT)


def build():
    prs = Presentation()
    prs.slide_width = SLIDE_W
    prs.slide_height = SLIDE_H
    blank = prs.slide_layouts[6]
    slides_meta = []

    def new_slide():
        s = prs.slides.add_slide(blank)
        add_bg(s, WHITE)
        slides_meta.append(s)
        return s

    total = 12  # fixed

    # 1 Title
    s = new_slide()
    # left accent panel
    panel = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, Inches(0.35), SLIDE_H)
    fill_shape(panel, TEAL)
    brand = add_textbox(s, Inches(0.9), Inches(2.0), Inches(11), Inches(1))
    set_run(brand.text_frame.paragraphs[0].add_run(), "ГеоАтлас", size=54, bold=True, color=INK)
    sub = add_textbox(s, Inches(0.9), Inches(3.05), Inches(11), Inches(0.9))
    set_run(
        sub.text_frame.paragraphs[0].add_run(),
        "Карта сетевых взаимодействий по логам межсетевых экранов",
        size=24,
        color=INK_SOFT,
    )
    meta = add_textbox(s, Inches(0.9), Inches(4.2), Inches(11), Inches(1.2))
    tf = meta.text_frame
    p = tf.paragraphs[0]
    set_run(p.add_run(), "Meetup для будущих пользователей  ·  до 20 минут + вопросы", size=16, color=MUTED)
    p2 = tf.add_paragraph()
    p2.space_before = Pt(8)
    set_run(p2.add_run(), "v2.7.2  ·  Apache 2.0  ·  open source", size=15, color=TEAL)

    # 2 Agenda
    s = new_slide()
    title_block(s, "О чём поговорим", "15 минут доклада + время на вопросы")
    items = [
        "Какая боль у тех, кто уже смотрит логи МСЭ",
        "Что даёт ГеоАтлас за первый час работы",
        "Рабочий день оператора: карта → аномалия → разбор",
        "Коротко про архитектуру (без глубокого dive)",
        "Как поставить и с чего начать",
        "Честные ограничения и Q&A",
    ]
    bullet_block(s, MARGIN, Inches(1.6), Inches(11.5), Inches(5), items, size=20)

    # 3 Problem
    s = new_slide()
    title_block(s, "Знакомая ситуация", "Логи есть. Картины — часто нет.")
    card(
        s,
        MARGIN,
        Inches(1.55),
        Inches(3.8),
        Inches(4.4),
        "В SIEM / syslog",
        [
            "Миллионы строк от UserGate,",
            "FortiGate, Cisco ASA / FTD",
            "Сложно увидеть «кто с кем»",
            "География и связи — в голове",
        ],
        accent=ACCENT_WARM,
    )
    card(
        s,
        Inches(4.75),
        Inches(1.55),
        Inches(3.8),
        Inches(4.4),
        "В голове аналитика",
        [
            "Ручной grep и pivot",
            "Новая страна? Репутация?",
            "Скан / beacon / lateral —",
            "часто ловятся поздно",
        ],
        accent=ACCENT_WARM,
    )
    card(
        s,
        Inches(8.8),
        Inches(1.55),
        Inches(3.8),
        Inches(4.4),
        "Что хочется",
        [
            "Живая карта связей",
            "Подсветка подозрительного",
            "Быстрый путь к разбору",
            "Без тяжёлого внедрения",
        ],
        accent=TEAL,
    )

    # 4 Solution / value
    s = new_slide()
    title_block(
        s,
        "ГеоАтлас — что это",
        "Appliance: принял syslog → разобрал → обогатил → показал на карте",
    )
    bullets = [
        "Принимает syslog с МСЭ (или из SIEM) на порт 514",
        "Парсит UserGate, FortiGate, Cisco ASA / FTD (+ generic fallback)",
        "Обогащает IP геоданными и рисует связи на 2D-карте и 3D-глобусе",
        "Подсвечивает репутацию IP и аномалии (scan, beacon, lateral…)",
        "Даёт hunts и workspace разбора алерта — не только «красивую картинку»",
    ]
    bullet_block(s, MARGIN, Inches(1.55), Inches(12), Inches(4.2), bullets, size=19)
    note = add_textbox(s, MARGIN, Inches(5.9), Inches(12), Inches(0.6))
    set_run(
        note.text_frame.paragraphs[0].add_run(),
        "Ценность: быстрее понять контекст сетевого события, не открывая десятки raw-логов.",
        size=16,
        bold=True,
        color=TEAL_DARK,
    )

    # 5 Value for personas
    s = new_slide()
    title_block(s, "Кому и зачем", "Гипотетические пользователи на meetup")
    card(
        s,
        MARGIN,
        Inches(1.55),
        Inches(3.8),
        Inches(4.5),
        "SOC / аналитик",
        [
            "Карта вместо таблицы",
            "Алерт → разбор peers",
            "Saved hunts на повторы",
            "Меньше ручного pivot",
        ],
    )
    card(
        s,
        Inches(4.75),
        Inches(1.55),
        Inches(3.8),
        Inches(4.5),
        "Сетевой / ИБ инженер",
        [
            "Видно внешние связи",
            "Группировка /24, город,",
            "страна — масштаб обзора",
            "Тест парсеров под ваш МСЭ",
        ],
    )
    card(
        s,
        Inches(8.8),
        Inches(1.55),
        Inches(3.8),
        Inches(4.5),
        "Руководитель / NOC",
        [
            "Роль dashboard",
            "Живая картина периметра",
            "Быстрый onboarding",
            "Один сервер — один пакет",
        ],
    )

    # 6 Operator journey
    s = new_slide()
    title_block(s, "Рабочий сценарий", "От сигнала до понимания за минуты")
    steps = [
        ("1. Карта", "Смотрите дуги,\nфильтры и период"),
        ("2. Аномалия", "Баннер / список:\nscan, surge, beacon…"),
        ("3. Разбор", "Peers, assign/ack,\nшаблон поиска"),
        ("4. Hunt", "Сохранили запрос —\nзапуск по расписанию"),
    ]
    x = MARGIN
    for title, body in steps:
        box = s.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, x, Inches(2.0), Inches(2.7), Inches(3.2))
        fill_shape(box, SURFACE)
        box.line.color.rgb = LINE
        box.adjustments[0] = 0.1
        t = add_textbox(s, x + Inches(0.2), Inches(2.25), Inches(2.3), Inches(0.6))
        set_run(t.text_frame.paragraphs[0].add_run(), title, size=18, bold=True, color=TEAL)
        b = add_textbox(s, x + Inches(0.2), Inches(3.0), Inches(2.3), Inches(1.8))
        tf = b.text_frame
        tf.word_wrap = True
        for i, line in enumerate(body.split("\n")):
            p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
            set_run(p.add_run(), line, size=15, color=INK_SOFT)
        x += Inches(3.05)
    # arrows between - simple chevrons via text
    for ax in [Inches(3.55), Inches(6.6), Inches(9.65)]:
        a = add_textbox(s, ax, Inches(3.3), Inches(0.4), Inches(0.4))
        set_run(a.text_frame.paragraphs[0].add_run(), "→", size=22, bold=True, color=TEAL)

    # 7 Product features (value-oriented)
    s = new_slide()
    title_block(s, "Что даёт продукт", "Не «фичи ради фич», а рычаги для пользователя")
    card(
        s,
        MARGIN,
        Inches(1.5),
        Inches(5.9),
        Inches(2.35),
        "Карта и обзор",
        [
            "2D / 3D, автообновление 30с / 1 / 5 мин",
            "Группировка: IP, /24, город, страна",
            "Конструктор поиска и личные шаблоны",
            "Command palette Ctrl/⌘K",
        ],
    )
    card(
        s,
        Inches(6.85),
        Inches(1.5),
        Inches(5.9),
        Inches(2.35),
        "Детекции и разбор",
        [
            "port / horizontal scan, blocked & byte surge",
            "beaconing, lateral fanout, new country",
            "репутационный peer; ack / assign",
            "investigate workspace + saved hunts",
        ],
    )
    card(
        s,
        MARGIN,
        Inches(4.1),
        Inches(5.9),
        Inches(2.2),
        "Данные и качество ingest",
        [
            "Репутация: offline-списки и URL-фиды",
            "Тест парсеров и журнал ошибок",
            "GeoIP: загрузка и правка базы",
            "TTL / retention в UI",
        ],
    )
    card(
        s,
        Inches(6.85),
        Inches(4.1),
        Inches(5.9),
        Inches(2.2),
        "Эксплуатация",
        [
            "Один tar.gz для Ubuntu / OL / RHEL",
            "Роли: admin / operator / dashboard",
            "HTTPS, мониторинг, бэкап ClickHouse",
            "OpenSSF practices, open source",
        ],
    )

    # 8 Architecture brief
    s = new_slide()
    title_block(s, "Архитектура — коротко", "Один хост, сеть geoatlas. UI не ходит в ClickHouse напрямую.")
    flow = add_textbox(s, MARGIN, Inches(1.45), Inches(12), Inches(1.0))
    tf = flow.text_frame
    tf.word_wrap = True
    p = tf.paragraphs[0]
    set_run(
        p.add_run(),
        "Письмо:  МСЭ → syslog-ng :514 → backend :1514 → ClickHouse",
        size=17,
        bold=True,
        color=TEAL_DARK,
    )
    p2 = tf.add_paragraph()
    p2.space_before = Pt(6)
    set_run(
        p2.add_run(),
        "Чтение:  браузер → frontend (nginx) → backend :8080 /api → ClickHouse",
        size=17,
        bold=True,
        color=TEAL_DARK,
    )
    bullets = [
        "Ядро: clickhouse + backend + frontend; профили: syslog, stats, dozzle",
        "Снаружи только :80/:443 и syslog-порты; backend/CH внутри docker-сети",
        "stats-collector пишет метрики в CH и scrape’ит backend + syslog-ng",
        "Доставка syslog: at-most-once (drops видны на /system)",
    ]
    bullet_block(s, MARGIN, Inches(2.7), Inches(12), Inches(3.2), bullets, size=17)
    note = add_textbox(s, MARGIN, Inches(5.9), Inches(12), Inches(0.5))
    set_run(
        note.text_frame.paragraphs[0].add_run(),
        "Appliance-модель: проще внедрить и сопровождать, чем «ещё один кластер».",
        size=15,
        color=MUTED,
    )

    # 9 Getting started
    s = new_slide()
    title_block(s, "С чего начать", "Путь гипотетического пользователя до первой карты")
    steps = [
        "Поставить пакет geoatlas-X.Y.Z.tar.gz с GitHub Releases (Ubuntu 20.04+ / OL8+ / RHEL)",
        "Пройти пошаговую установку, задать пароль admin (≥ 8 символов)",
        "Открыть UI, загрузить GeoIP-базу",
        "Направить syslog МСЭ на :514 (желательно ограничить источник)",
        "На карте: период, группировка, фильтры — первая картина связей",
        "Задать сети предприятия → включить смысл аномалий; при желании — репутацию",
    ]
    bullet_block(s, MARGIN, Inches(1.55), Inches(12), Inches(4.8), steps, size=18)

    # 10 Honest limits
    s = new_slide()
    title_block(s, "Честно про границы", "Чтобы ожидания совпали с продуктом")
    card(
        s,
        MARGIN,
        Inches(1.55),
        Inches(5.9),
        Inches(4.5),
        "Сейчас так",
        [
            "IPv4-first (карта / GeoIP success path)",
            "Один хост / один процесс backend",
            "Syslog :514 без TLS по умолчанию —",
            "  защита сетью / allowlist",
            "Не замена полноценному SIEM:",
            "  фокус на связях и гео-контексте",
        ],
        accent=ACCENT_WARM,
    )
    card(
        s,
        Inches(6.85),
        Inches(1.55),
        Inches(5.9),
        Inches(4.5),
        "Зато удобно",
        [
            "Быстрый старт без тяжёлого проекта",
            "Понятный операторский UI",
            "Парсеры популярных МСЭ",
            "Детекции + hunts «из коробки»",
            "Бесплатно, Apache 2.0, открытый код",
            "Документация и releases на GitHub",
        ],
        accent=TEAL,
    )

    # 11 Takeaways
    s = new_slide()
    title_block(s, "Что унести с собой", "Три мысли для пользователей")
    big = [
        ("Смотрите связи, не только строки", "Карта и группировки дают контекст быстрее raw-лога."),
        ("От сигнала — к действию", "Аномалия → разбор → hunt: замкнутый рабочий цикл."),
        ("Малый порог входа", "Один пакет, свой syslog, свой GeoIP — можно попробовать на стенде."),
    ]
    y = Inches(1.6)
    for t, d in big:
        n = add_textbox(s, MARGIN, y, Inches(12), Inches(0.4))
        set_run(n.text_frame.paragraphs[0].add_run(), t, size=22, bold=True, color=TEAL)
        n2 = add_textbox(s, MARGIN, y + Inches(0.4), Inches(12), Inches(0.45))
        set_run(n2.text_frame.paragraphs[0].add_run(), d, size=16, color=INK_SOFT)
        y += Inches(1.35)

    # 12 Q&A
    s = new_slide()
    panel = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, Inches(0.35), SLIDE_H)
    fill_shape(panel, TEAL)
    t = add_textbox(s, Inches(0.9), Inches(2.2), Inches(11), Inches(1))
    set_run(t.text_frame.paragraphs[0].add_run(), "Вопросы?", size=48, bold=True, color=INK)
    sub = add_textbox(s, Inches(0.9), Inches(3.3), Inches(11), Inches(1.5))
    tf = sub.text_frame
    p = tf.paragraphs[0]
    set_run(p.add_run(), "GitHub: github.com/varlahin-gena/geoatlas", size=18, color=INK_SOFT)
    p2 = tf.add_paragraph()
    p2.space_before = Pt(10)
    set_run(p2.add_run(), "Docs: docs/ в репозитории  ·  Issues: баги и идеи", size=18, color=INK_SOFT)
    p3 = tf.add_paragraph()
    p3.space_before = Pt(10)
    set_run(p3.add_run(), "Лицензия Apache 2.0 — можно ставить и пробовать", size=18, color=TEAL)

    # footers (skip title and qa for cleaner look? add to content slides 2-11)
    for i, slide in enumerate(slides_meta, start=1):
        if i not in (1, 12):
            add_footer(slide, i, total)

    out = Path(__file__).resolve().parent / "GeoAtlas-meetup.pptx"
    # If the file is open in PowerPoint/IDE, write a sibling and keep trying primary.
    try:
        prs.save(out)
        print(out)
        return out
    except PermissionError:
        alt = out.with_name("GeoAtlas-meetup-v2.pptx")
        prs.save(alt)
        print(f"locked: {out}\nwrote: {alt}")
        return alt


if __name__ == "__main__":
    build()
