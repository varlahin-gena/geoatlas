import type { AnomalyThresholds } from '@/api/anomalies';

type ThresholdFieldKey = keyof AnomalyThresholds;

export type ThresholdFieldMeta = {
  key: ThresholdFieldKey;
  label: string;
  hint?: string;
  min?: number;
  max?: number;
  step?: number;
};

export type DetectorThresholdMeta = {
  id: string;
  code: string;
  title: string;
  window: string;
  summary: string;
  advice: string;
  fields: ThresholdFieldMeta[];
};

export type DetectorGroupMeta = {
  id: string;
  title: string;
  blurb: string;
  detectors: DetectorThresholdMeta[];
};

/** Группы порогов: сканирование → всплески → поведение → гео/репутация. */
export const DETECTOR_THRESHOLD_GROUPS: DetectorGroupMeta[] = [
  {
    id: 'scanning',
    title: 'Сканирование',
    blurb: 'Много целей или портов за короткое окно — типичный разведка/сканер.',
    detectors: [
      {
        id: 'port_scan',
        code: 'port_scan',
        title: 'Сканирование портов',
        window: '5 мин',
        summary:
          'Один источник за 5 минут обращается к большому числу разных портов назначения. Оба порога должны выполниться одновременно.',
        advice:
          'Больше ложных срабатываний на шумном периметре или при сканерах уязвимостей — поднимите «Портов» и «Событий» (профиль small: 40 / 80). На тихой сети можно снизить. Если нужны сканы с RFC1918 — включите «Учитывать частные IP» выше.',
        fields: [
          {
            key: 'port_scan_ports',
            label: 'Уникальных портов',
            hint: 'минимум разных dst_port',
          },
          {
            key: 'port_scan_events',
            label: 'Событий',
            hint: 'минимум записей в окне',
          },
        ],
      },
      {
        id: 'horizontal_scan',
        code: 'horizontal_scan',
        title: 'Сканирование подсети',
        window: '5 мин',
        summary:
          'Источник за 5 минут стучится во многие хосты одной подсети /24. Оба порога обязательны.',
        advice:
          'Поднимите «Хостов» / «Событий», если в сети много легитимных опросов (мониторинг, AD, сканеры активов). Для малого периметра профиль small (30 / 60) обычно достаточен.',
        fields: [
          {
            key: 'horizontal_hosts',
            label: 'Хостов в /24',
            hint: 'уникальных назначений',
          },
          {
            key: 'horizontal_events',
            label: 'Событий',
            hint: 'минимум записей в окне',
          },
        ],
      },
      {
        id: 'lateral_fanout',
        code: 'lateral_fanout',
        title: 'Веер по сети предприятия',
        window: '15 мин',
        summary:
          'Источник из enterprise-сети за 15 минут обращается ко многим внутренним хостам (east-west). Источник и назначения должны быть в сетях предприятия.',
        advice:
          'Чаттивые сервисы (оркестраторы, бэкапы, SIEM-агенты) дают веер — поднимите пороги. Для компрометации ищут резкий рост уникальных внутренних dst; профиль small: 20 хостов / 40 событий.',
        fields: [
          {
            key: 'lateral_hosts',
            label: 'Внутренних хостов',
            hint: 'уникальных dst в enterprise',
          },
          {
            key: 'lateral_events',
            label: 'Событий',
            hint: 'минимум записей в окне',
          },
        ],
      },
    ],
  },
  {
    id: 'surges',
    title: 'Всплески',
    blurb: 'Сравнение текущего окна с предыдущим: нужен и кратный рост, и абсолютный минимум.',
    detectors: [
      {
        id: 'blocked_surge',
        code: 'blocked_surge',
        title: 'Всплеск блокировок',
        window: '15 мин',
        summary:
          'По каждой enterprise-сети: блокировки за последние 15 мин сравниваются с предыдущими 15 мин. Срабатывает, если текущее ≥ max(предыдущее × Ratio, Min событий), и предыдущее уже ≥ Floor. Повтор по той же сети подавляется на 6 ч.',
        advice:
          'Floor отсекает тихие сети. Ratio 4–5 — баланс; Min событий не даёт алертить на малых абсолютах. Много шума после смены правил FW — временно поднимите Min и Floor. Severity high, если текущее ≥ 5× Min.',
        fields: [
          {
            key: 'surge_ratio',
            label: 'Множитель (×)',
            hint: 'относительно прошлого окна',
            min: 1,
            step: 0.1,
          },
          {
            key: 'surge_abs_min',
            label: 'Мин. событий',
            hint: 'абсолютный порог срабатывания',
          },
          {
            key: 'surge_floor',
            label: 'Floor прошлого окна',
            hint: 'ниже — сеть не сравниваем',
          },
        ],
      },
      {
        id: 'byte_surge',
        code: 'byte_surge',
        title: 'Всплеск объёма',
        window: '1 ч',
        summary:
          'По источнику: байты за текущий час vs предыдущий. Та же схема Ratio / Min / Floor, но в байтах, не в событиях.',
        advice:
          'Окна бэкапов и синхронизаций — поднимите Min байт и Floor. Ночью на quiet-сети можно оставить профиль. Профиль small: Ratio 5×, Min 80 МБ, Floor 2 МБ. High — при очень больших абсолютах (как у blocked_surge).',
        fields: [
          {
            key: 'byte_surge_ratio',
            label: 'Множитель (×)',
            hint: 'относительно прошлого часа',
            min: 1,
            step: 0.1,
          },
          {
            key: 'byte_surge_abs_min',
            label: 'Мин. байт',
            hint: 'например 80000000 ≈ 80 МБ',
          },
          {
            key: 'byte_surge_floor',
            label: 'Floor прошлого часа',
            hint: 'байт; ниже — src пропускаем',
          },
        ],
      },
    ],
  },
  {
    id: 'behavior',
    title: 'Поведение',
    blurb: 'Долгие слабые, но регулярные связи — похожи на beacon / C2.',
    detectors: [
      {
        id: 'beaconing',
        code: 'beaconing',
        title: 'Периодическая связь',
        window: '24 ч',
        summary:
          'Пара src→dst активна не меньше Min часов за сутки, средний объём в час не выше Max avg, а регулярность часовых интервалов ≥ Regularity (0…1). Fingerprint суточный — без спама каждый тик. Во время обучения детектор пропускается.',
        advice:
          'Строже к C2-подобному: выше Min часов и Regularity, ниже Max avg. Мониторинг/NTP/heartbeat с ровным ритмом — ложные; поднимите Max avg или Regularity. Профиль small: 10 ч / 250 КБ/ч / 0.55.',
        fields: [
          {
            key: 'beacon_min_hours',
            label: 'Мин. активных часов',
            hint: 'из 24',
            max: 168,
          },
          {
            key: 'beacon_max_avg_bytes',
            label: 'Макс. ср. байт/час',
            hint: 'выше — не считаем low-volume',
          },
          {
            key: 'beacon_min_regularity',
            label: 'Регулярность',
            hint: '0…1, ближе к 1 — ровнее',
            min: 0,
            max: 1,
            step: 0.01,
          },
        ],
      },
    ],
  },
  {
    id: 'geo_rep',
    title: 'География и репутация',
    blurb: 'Новые страны назначения и первые контакты с известными плохими адресами.',
    detectors: [
      {
        id: 'new_country',
        code: 'new_country_dst',
        title: 'Новая страна назначения',
        window: '1 ч vs 7 дн',
        summary:
          'Страна назначения не встречалась в 7-дневном baseline (с порогом Baseline), в текущем часе ≥ Min событий, и доля трафика в эту страну ≥ Min share. Во время обучения baseline детектор пропускается.',
        advice:
          'Min share (например 0.05 = 5%) отсекает единичные запросы. Частые ложные на CDN/VPN — поднимите Min событий и Baseline. Доля и минимумы почти не зависят от профиля (кроме xlarge).',
        fields: [
          {
            key: 'new_country_min',
            label: 'Мин. событий (час)',
            hint: 'в текущем окне',
          },
          {
            key: 'new_country_baseline',
            label: 'Порог baseline',
            hint: 'минимум за 7 дней, чтобы страна считалась «известной»',
          },
          {
            key: 'new_country_min_share',
            label: 'Мин. доля',
            hint: '0.05 = 5% трафика часа',
            min: 0.01,
            max: 1,
            step: 0.01,
          },
        ],
      },
      {
        id: 'reputation_peer',
        code: 'rep_new_peer',
        title: 'Репутационная связь',
        window: '15 мин',
        summary:
          'Рёбра за 15 минут с попаданием в репутационный список: пара src↔dst ещё не встречалась в lookback ~7 дней, и число событий по ребру ≥ Min событий.',
        advice:
          'Единичные DNS/сканы — поднимите Min событий (3 по умолчанию). Критичные списки лучше оставлять чувствительными: ложное лучше пропустить, чем пропустить первый контакт.',
        fields: [
          {
            key: 'rep_min_events',
            label: 'Мин. событий по ребру',
            hint: 'за окно 15 мин',
          },
        ],
      },
    ],
  },
];
