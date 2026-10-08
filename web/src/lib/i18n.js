// Every string that differs by locale. Imported on the server for markup and in
// the browser for status messages, so it stays one source of truth.
export const locales = ['en', 'ja'];
export const defaultLocale = 'en';

export const strings = {
  en: {
    htmlLang: 'en',
    siteName: '天気Now',
    nav: { overview: 'Overview', map: 'Map', playground: 'Playground', reference: 'Reference' },
    skip: 'Skip to content',
    other: { code: 'ja', label: '日本語', path: (p) => `/ja${p}` },
    mast: { line: 'AMeDAS · updated every ten minutes · JST' },

    home: {
      // The head term first, then the modifier people actually type ("free"),
      // then the specialist term. The brand goes last: nobody searching for a
      // weather API types 天気Now, and in first position it spends the whole
      // width budget on a word only existing users know.
      title: 'Japan weather API: free current temperature as JSON | 天気Now',
      description:
        "Free JSON API for the current temperature, rainfall and wind anywhere in Japan, no API key. Send a coordinate, get the nearest JMA AMeDAS station's reading. Ten-minute updates, 30 days of history.",
      h1: [
        'A free weather API for Japan.',
        'Current temperature from any AMeDAS station, by coordinate.',
      ],
      lede: "The Japan Meteorological Agency publishes fresh observations every ten minutes from about 1,300 automated stations. This service polls them, cleans up the readings, and hands you the nearest station's data as JSON for any latitude and longitude you ask about.",
      nowLabel: 'Nearest station, now',
      locate: 'Read my nearest station',
      orCoord: 'or a coordinate',
      read: 'Read',
      coordHelp: 'Latitude, longitude. Anywhere in Japan.',
      stationIdle: '観測所',
      stationIdleEn: 'waiting for a coordinate',
      bucketIdle: '10-minute bucket, JST',
      fields: {
        humidity: 'Humidity',
        pressure: 'Pressure',
        wind: 'Wind',
        rain: 'Rain, 1h',
        sun: 'Sun, 1h',
        snow: 'Snow depth',
      },
      note: "A dash means the station does not measure that, or the reading failed JMA's quality check. The key is still there in the JSON, set to",
      endpoints: [
        {
          path: '/v1/weather/latest',
          params: 'lat, lon',
          blurb: 'The newest reading from the station nearest a coordinate.',
        },
        {
          path: '/v1/weather/history',
          params: 'lat, lon, from, to',
          blurb:
            'Every reading in a time range. We keep 30 days, so a from earlier than that gets clamped and the response tells you.',
        },
        {
          path: '/v1/weather/at',
          params: 'lat, lon, at',
          blurb:
            'The reading that was current at a given moment. Floored to the ten-minute bucket it falls in, so 14:23 returns 14:20, never 14:30.',
        },
      ],
      apiH2: 'How to call it',
      apiLede:
        'Plain HTTP and JSON. There is no key and no account to set up: one GET with a coordinate.',
      apiEndpoints: 'The endpoints',
      apiExample: 'A request, and what comes back',
      apiTrimmed:
        'Trimmed for space. The full field list and the error shapes are in the reference.',
      facts: [
        { k: 'No key', v: 'Public unauthenticated GETs. Nothing to sign up for.' },
        {
          k: 'JSON over HTTPS',
          v: 'Timestamps are RFC3339 UTC, and units live in the field names.',
        },
        { k: '30 days', v: 'Of ten-minute history kept for every station.' },
        { k: 'Attributed', v: 'Every response carries its JMA attribution.' },
      ],
      conventions:
        'Timestamps are RFC3339 UTC, units live in the field names, and every response carries the JMA attribution. A coordinate more than 100 km from any station returns 404: it is offshore or outside Japan, and the nearest station is too far away to tell you anything about it.',
      tryIt: 'Try them in the playground',
      readRef: 'read the OpenAPI reference',
      or: 'or',

      // The six questions every first-time reader arrives with. They are here
      // because they are also the six things someone types into a search box
      // ("is it free", "does it do forecasts", "how often does it update"), and
      // because an answer engine quoting this page will quote one of these.
      faqH2: 'Questions people ask first',
      faq: [
        {
          q: 'Can I get the current temperature from it?',
          a: "Yes, that is the main thing it does. One GET with a latitude and longitude returns the current temperature at the nearest AMeDAS station, alongside rainfall, wind, humidity and pressure, as JSON. The reading is the station's last ten-minute observation, usually a few minutes old.",
        },
        {
          q: 'Is it free, and do I need an API key?',
          a: 'It is free, and there is no key, no account and no SDK. Every endpoint is a public GET over HTTPS. The only limit is an edge rule against runaway clients: 50 requests every 10 seconds per IP address, after which the edge answers 429 for ten seconds.',
        },
        {
          q: 'Does it return weather forecasts?',
          a: 'No, observed data only. The Japan Meteorological Agency publishes forecasts separately and this service does not mirror them. For forecasts and official warnings, use jma.go.jp.',
        },
        {
          q: 'How fresh is the data, and how far back does it go?',
          a: 'Readings land on ten-minute boundaries and JMA is polled every five minutes, so a reading is usually available within a few minutes of the boundary it belongs to. Thirty days of history is kept for every station; anything older is dropped.',
        },
        {
          q: 'What area does it cover?',
          a: 'Japan, through roughly 1,300 AMeDAS stations. A coordinate more than 100 km from any station returns 404: it is offshore or outside the country, and the nearest station is too far away to say anything useful about it.',
        },
        {
          q: 'Is this point data for the coordinate I send?',
          a: "No. Every response names the station it resolved to and how far away that station was. A coordinate 40 km from the nearest station gets that station's measured reading, not an interpolated estimate for your spot.",
        },
        {
          q: 'Can I use it in a commercial product?',
          a: "The observations are JMA's, published under the Public Data License v1.0, which is compatible with CC BY 4.0, so yes, as long as you carry the attribution that ships in every response. The service itself is unofficial, provided as is with no warranty, and not for safety-critical use.",
        },
      ],
    },

    map: {
      title: 'Live AMeDAS map: temperature, rain and wind in Japan | 天気Now',
      description:
        'Every reporting AMeDAS station in Japan on one map, coloured by temperature, rainfall or wind, refreshed every ten minutes.',
      h2: 'The whole network, right now',
      lede: 'One dot per reporting station, placed at its actual coordinates and coloured by what it is measuring right now. Click a dot, or search for a station by name.',
      modes: { temp: 'Temperature', rain: 'Rain, 1h', wind: 'Wind' },
      units: { temp: '°C', rain: 'mm', wind: 'm/s' },
      find: 'Find a station',
      findHelp: 'Type a station name to read it without hunting for it on the map.',
      loading: 'Loading the network…',
      counted: (n, when) => `${n} stations reporting · newest reading ${when} JST`,
      failed: 'Could not load the snapshot. The API may be down.',
      hint: 'Click a dot to read a station.',
      noRain: 'No rain',
      calm: 'Calm',
      endpointNote: 'This page is one call to',
    },

    pg: {
      // "Playground" alone is a word nobody searches. The page keeps the name
      // in the nav and spends the title on what it is for.
      title: 'API playground: try the Japan weather API live | 天気Now',
      description:
        'Build a request against the free Japan weather API and see the JSON it returns, from your own coordinate or a preset Japanese city. No key, no signup.',
      h2: 'Playground',
      lede: 'Pick an endpoint, set a coordinate, send it. The request runs from your browser against the live API, so what you see here is what your code will get.',
      endpoint: 'Endpoint',
      coordinate: 'Coordinate',
      here: 'use my location',
      coordHelp:
        'Leave both blank and the API works out roughly where you are from your IP address.',
      range: 'Range',
      rangeHelp: 'Sent as UTC. We keep 30 days, so anything earlier comes back clamped.',
      moment: 'Moment',
      momentHelp:
        'Floored to the ten-minute bucket it falls in. You get a 404 if the station was silent for the 30 minutes before it.',
      send: 'Send request',
      copy: 'Copy as curl',
      copied: 'Copied',
      copiedSaid: 'Copied the curl command to the clipboard.',
      notSent: 'Not sent yet.',
    },

    msg: {
      located: 'Location set. Send the request.',
      outside: 'No station within 100 km. That coordinate is outside Japan.',
      apiSaid: (code) => `The API answered ${code}.`,
      unreachable: (host) => `Could not reach ${host}.`,
      networkError: 'could not reach api.tenkinow.com',
      badCoord: 'Write it as "35.69, 139.70".',
      fromNetwork:
        'Located from your network, so it is only rough. Type a coordinate to correct it.',
      away: (km, m) => `${km} km away · ${m} m`,
      blocked: 'Location is blocked. Allow it in the address bar, or type a coordinate.',
      noLocation: 'Could not read a location. Type a coordinate instead.',
      noGeo: 'This browser cannot share a location.',
    },

    nf: {
      title: 'Page not found (404) | 天気Now',
      description: 'That page does not exist on tenkinow.com.',
      h1: ['404.', 'Nothing at this address.'],
      lede: 'The page you asked for is not here. Everything this site has is one of these.',
    },

    station: {
      kicker: 'AMeDAS station',
      title: (s) =>
        `${s.name_en} (${s.name}) current temperature: AMeDAS station observations | 天気Now`,
      description: (s) =>
        `Current temperature, rainfall and wind at the ${s.name_en} (${s.name}) AMeDAS station, ${s.altitude_m} m above sea level at ${s.lat}, ${s.lon}. Read it as JSON from the free 天気Now API, no key needed.`,
      lede: (s) =>
        `Station ${s.id} in the Japan Meteorological Agency's AMeDAS network. It reports on every ten-minute boundary, and the call below returns whatever it last sent.`,
      live: 'Latest reading',
      liveNote: 'Read live from the API when this page loaded.',
      id: 'Station id',
      coord: 'Coordinates',
      alt: 'Altitude',
      callH2: 'Read this station from the API',
      callNote:
        'The API resolves a coordinate to the nearest station, so asking with this station\u2019s own coordinate returns this station. Swap /latest for /history or /at to read further back.',
      nearH2: 'Nearest other stations',
      away: (km) => `${km} km away`,
      onMap: 'Find it on the live map',
      tryIt: 'Open it in the playground',
      indexTitle: 'Every AMeDAS station in Japan: the full list | 天気Now',
      indexDescription:
        'All 1,286 Japan Meteorological Agency AMeDAS weather stations, with coordinates, altitude and station id, each one readable as JSON from the free 天気Now API.',
      indexH1: ['Every AMeDAS station', 'in Japan.'],
      indexLede: (n) =>
        `All ${n} stations the Japan Meteorological Agency reports from, sorted by station id, which runs roughly north to south, Hokkaido first.`,
      all: 'All stations',
    },

    docs: {
      title: 'Japan weather API reference: endpoints, fields and errors | 天気Now',
      description:
        'Full reference for the free 天気Now weather API: every endpoint, query parameter, response field and error shape, rendered from the OpenAPI 3.1 spec.',
      h1: ['API reference.', 'Every endpoint and field.'],
      lede: 'Rendered from the same OpenAPI 3.1 document the API serves, so this page cannot drift from the thing it describes.',
      spec: 'OpenAPI 3.1 document',
      interactive: 'Interactive reference',
      params: 'Query parameters',
      required: 'required',
      optional: 'optional',
      responses: 'Responses',
      eg: (v) => `e.g. ${v}`,
      arrayOf: (t) => `array of ${t}`,
      schemas: 'Response fields',
      errors: 'Errors',
      noParams: 'No parameters.',
      limitsH2: 'Rate limits',
      limits: [
        'There is no key and no per-caller quota. One edge rule stands between the API and a client stuck in a loop: `50` requests every `10` seconds, counted per IP address per Cloudflare location, across every `/v1/weather/` path.',
        'Go over it and the edge answers `429` until the ten-second window clears; nothing is banned for longer than that. The count is taken before the cache is consulted, so responses served from the edge count the same as ones that reach the origin. If you are fanning out over many stations at once, keep the burst under fifty. These numbers describe how the service is run today, not a guarantee: the limit, and the endpoints themselves, may change without notice, and the service is provided as is with no uptime commitment.',
      ],
    },

    colophon: {
      tagline: 'Nearest-station observations for Japan, served as JSON.',
      facts: [
        ['10 min', 'refresh cadence'],
        ['30 days', 'history kept'],
        ['~1,300', 'stations covered'],
        ['JMA PDL 1.0', 'data licence'],
      ],
      contact: 'Questions, licence or takedown',
      security: 'Report a vulnerability',
      source: 'Source on GitHub',
    },

    footer: [
      '出典：気象庁ホームページ (',
      ') を加工して作成（編集責任：tenkinow）. Observations come from the Japan Meteorological Agency under the Public Data License v1.0 (公共データ利用規約第1.0版), normalized rather than republished as JMA issues it. Unofficial and not endorsed by JMA: observed data only, no forecasts. The upstream feed may lag, gap, change or stop without notice, so this service may serve stale or incomplete data, or none at all. Provided as is, with no warranty. Not for safety-critical use. For official warnings and forecasts, see the JMA site.',
    ],
  },

  ja: {
    htmlLang: 'ja',
    siteName: '天気Now',
    nav: {
      overview: '概要',
      map: 'マップ',
      playground: 'プレイグラウンド',
      reference: 'リファレンス',
    },
    skip: '本文へスキップ',
    other: { code: 'en', label: 'English', path: (p) => p },
    mast: { line: 'アメダス・10分ごと更新・日本時間' },

    home: {
      // 「気象庁 API」「天気 API 無料」が実際に打たれる語。ブランド名は末尾。
      title: '現在の気温をJSONで返す無料の天気API — 気象庁アメダス観測値 | 天気Now',
      description:
        '現在の気温・降水量・風速を気象庁アメダスの観測値からJSONで返す無料APIです。APIキーは不要。緯度・経度を送ると最寄り観測所の値が返ります。10分ごとに更新、直近30日分を保持。',
      h1: ['全国の現在の気温を、', '緯度・経度で引く無料API。'],
      lede: '気象庁は約1,300か所の自動観測所から10分ごとに新しい観測値を公開しています。このサービスはそれを取り込んで正規化し、緯度・経度に対して最寄り観測所のデータをJSONで返します。',
      nowLabel: '最寄り観測所・現在',
      locate: '最寄りの観測所を読む',
      orCoord: 'または緯度・経度',
      read: '読む',
      coordHelp: '緯度・経度。日本国内ならどこでも。',
      stationIdle: '観測所',
      stationIdleEn: '緯度・経度の入力待ち',
      bucketIdle: '10分値・日本時間',
      fields: {
        humidity: '湿度',
        pressure: '気圧',
        wind: '風',
        rain: '降水量 1時間',
        sun: '日照 1時間',
        snow: '積雪深',
      },
      note: 'ダッシュは、その観測所がその要素を観測していないか、品質情報が正常でなかったことを示します。JSONのキーは常に存在し、値は',
      endpoints: [
        {
          path: '/v1/weather/latest',
          params: 'lat, lon',
          blurb: '指定座標に最も近い観測所の、最新の観測値。',
        },
        {
          path: '/v1/weather/history',
          params: 'lat, lon, from, to',
          blurb:
            '期間内のすべての観測値。保持は30日間で、それより前のfromは切り詰められ、その旨がレスポンスに入ります。',
        },
        {
          path: '/v1/weather/at',
          params: 'lat, lon, at',
          blurb:
            '指定時刻に有効だった観測値。10分の区切りに切り下げるため、14:23は14:20を返し、14:30は返しません。',
        },
      ],
      apiH2: 'APIの呼び方',
      apiLede: 'ふつうのHTTPとJSON。キーもアカウントも要りません。GETひとつと緯度・経度だけです。',
      apiEndpoints: 'エンドポイント',
      apiExample: 'リクエストと、返ってくるもの',
      apiTrimmed:
        '紙幅の都合で一部を省略しています。全フィールドとエラー形式はリファレンスにあります。',
      facts: [
        { k: 'キー不要', v: '認証なしの公開GET。登録するものはありません。' },
        { k: 'HTTPS + JSON', v: '時刻はRFC3339のUTC、単位はフィールド名に含まれます。' },
        { k: '30日間', v: '各観測所の10分値を30日分保持しています。' },
        { k: '出典表示', v: 'すべてのレスポンスに気象庁の出典表示が付きます。' },
      ],
      conventions:
        '時刻はRFC3339のUTC、単位はフィールド名に含まれ、すべてのレスポンスに気象庁の出典表示が付きます。どの観測所からも100kmを超える座標は404です。洋上か国外で、いちばん近い観測所でも遠すぎて、その地点について何も言えないからです。',
      tryIt: 'プレイグラウンドで試す',
      readRef: 'OpenAPIリファレンスを読む',
      or: 'または',

      // 検索窓に実際に打たれる六つの問い。回答エンジンがこのページを引くとき、
      // 引かれるのはここのどれかになる。
      faqH2: 'よくある質問',
      faq: [
        {
          q: '現在の気温をAPIで取得できますか。',
          a: 'はい。それがこのAPIの主な用途です。緯度・経度を付けてGETを1回投げるだけで、最寄りのアメダス観測所の現在の気温が、降水量・風速・湿度・気圧とあわせてJSONで返ります。値はその観測所の直近10分値で、通常は数分前のものです。',
        },
        {
          q: '無料ですか。APIキーは必要ですか。',
          a: '無料で、キーもアカウントもSDKも不要です。すべてのエンドポイントはHTTPS越しの公開GETです。制限は暴走したクライアント対策のエッジ側のルールだけで、IPアドレスごとに10秒あたり50リクエスト、超えると10秒間 429 を返します。',
        },
        {
          q: '天気予報は返りますか。',
          a: '返しません。提供するのは観測値のみです。予報は気象庁が別に発表しており、本サービスはそれを転載しません。予報・警報は jma.go.jp をご確認ください。',
        },
        {
          q: 'データの鮮度と、さかのぼれる期間は。',
          a: '観測値は10分の区切りに並び、気象庁を5分ごとに取り込んでいるため、通常はその区切りの数分後には取得できます。履歴は全観測所について30日分を保持し、それより古いものは削除します。',
        },
        {
          q: '対象範囲はどこまでですか。',
          a: '日本国内、約1,300か所のアメダス観測所です。どの観測所からも100kmを超える座標は404を返します。洋上か国外で、いちばん近い観測所でも遠すぎるからです。',
        },
        {
          q: '送った座標そのものの値が返るのですか。',
          a: 'いいえ。ここは重要な違いです。レスポンスには必ず、解決した観測所とそこまでの距離が入ります。最寄り観測所から40km離れた座標には、その観測所の実測値が返ります。地点内挿の推定値ではありません。',
        },
        {
          q: '商用利用できますか。',
          a: '元の観測値は気象庁が公共データ利用規約（第1.0版）で公開しているもので、CC BY 4.0と互換です。各レスポンスに含まれる出典表示を一緒に掲示すれば、商用でも利用できます。ただし本サービス自体は非公式・無保証であり、防災など安全に関わる用途には使わないでください。',
        },
      ],
    },

    map: {
      title: 'アメダスライブマップ — 全国の気温・降水量・風 | 天気Now',
      description:
        '全国の観測中アメダス観測所を1枚の地図に。気温・降水量・風で色分けし、10分ごとに更新します。',
      h2: '観測網の、いまの姿',
      lede: '観測中の観測所を1点ずつ、実際の座標に、いま測っている値で色分けして表示しています。点をクリックするか、観測所名で検索してください。',
      modes: { temp: '気温', rain: '降水量 1時間', wind: '風' },
      units: { temp: '°C', rain: 'mm', wind: 'm/s' },
      find: '観測所を探す',
      findHelp: '地図を使わずに読むには、観測所名を入力してください。',
      loading: '観測網を読み込んでいます…',
      counted: (n, when) => `${n}か所が観測中・最新の観測値 ${when}`,
      failed: 'スナップショットを読み込めませんでした。APIが停止している可能性があります。',
      hint: '点をクリックすると観測所を読めます。',
      noRain: '降水なし',
      calm: '静穏',
      endpointNote: 'このページが呼んでいるのは',
    },

    pg: {
      title: 'APIプレイグラウンド — 天気APIをブラウザで試す | 天気Now',
      description:
        '無料の天気Now APIへのリクエストを組み立てて、返ってくるJSONをその場で確認できます。APIキー不要。現在地でも、主要都市のプリセットでも。',
      h2: 'プレイグラウンド',
      lede: 'エンドポイントを選び、座標を入れて送信します。リクエストはブラウザから本番APIに直接飛ぶので、ここで見えるものがそのままコードに返ります。',
      endpoint: 'エンドポイント',
      coordinate: '座標',
      here: '現在地を使う',
      coordHelp: '両方とも空にすると、APIがIPアドレスから推定した位置を使います。',
      range: '期間',
      rangeHelp: 'UTCで送信します。保持は30日間なので、それより前のfromは切り詰められます。',
      moment: '時刻',
      momentHelp: '10分の区切りに切り下げます。その30分前まで観測値がない場合は404になります。',
      send: 'リクエストを送信',
      copy: 'curlをコピー',
      copied: 'コピーしました',
      copiedSaid: 'curlコマンドをクリップボードにコピーしました。',
      notSent: '未送信',
    },

    msg: {
      located: '現在地を設定しました。リクエストを送信してください。',
      outside: '100km以内に観測所がありません。この座標は日本国外です。',
      apiSaid: (code) => `APIが ${code} を返しました。`,
      unreachable: (host) => `${host} に接続できませんでした。`,
      networkError: 'api.tenkinow.com に接続できませんでした',
      badCoord: '「35.69, 139.70」の形式で入力してください。',
      fromNetwork:
        'ネットワークからの推定位置です。大まかなので、正確にするには座標を入力してください。',
      away: (km, m) => `${km} km・標高 ${m} m`,
      blocked: '位置情報がブロックされています。アドレスバーで許可するか、座標を入力してください。',
      noLocation: '位置情報を取得できませんでした。座標を入力してください。',
      noGeo: 'このブラウザは位置情報を共有できません。',
    },

    nf: {
      title: 'ページが見つかりません — 404 | 天気Now',
      description: 'そのページは tenkinow.com にありません。',
      h1: ['404。', 'このアドレスには何もありません。'],
      lede: 'お探しのページはありません。このサイトにあるのは、次のページです。',
    },

    station: {
      kicker: 'アメダス観測所',
      title: (s) => `${s.name}の現在の気温 — アメダス観測所（${s.name_en}）の観測値 | 天気Now`,
      description: (s) =>
        `${s.name}（${s.name_en}）アメダス観測所の現在の気温・降水量・風速。標高${s.altitude_m}m、${s.lat}, ${s.lon}。無料の天気Now APIからJSONで取得できます。APIキーは不要。`,
      lede: (s) =>
        `気象庁アメダス観測網の観測所番号 ${s.id}。10分ごとに観測値を送信しており、下のリクエストはその最新値を返します。`,
      live: '最新の観測値',
      liveNote: 'このページを開いた時点でAPIから取得した値です。',
      id: '観測所番号',
      coord: '緯度・経度',
      alt: '標高',
      callH2: 'この観測所をAPIで読む',
      callNote:
        'APIは座標を最寄り観測所に解決するため、この観測所自身の座標で問い合わせればこの観測所が返ります。/latest を /history や /at に替えれば過去もたどれます。',
      nearH2: '近くの観測所',
      away: (km) => `${km} km`,
      onMap: 'ライブマップで見る',
      tryIt: 'プレイグラウンドで開く',
      indexTitle: '全国のアメダス観測所一覧 | 天気Now',
      indexDescription:
        '気象庁アメダス観測所1,286か所の一覧。緯度・経度、標高、観測所番号つき。それぞれ無料の天気Now APIからJSONで取得できます。',
      indexH1: ['全国のアメダス観測所', '一覧。'],
      indexLede: (n) =>
        `気象庁が観測値を発表している${n}か所すべて。観測所番号順で、番号はおおむね北から南へ、北海道から並びます。`,
      all: '観測所一覧',
    },

    docs: {
      title: '天気APIリファレンス — エンドポイント・フィールド・エラー | 天気Now',
      description:
        '無料の天気Now APIの完全なリファレンス。全エンドポイント、クエリパラメータ、レスポンスフィールド、エラー形式を、OpenAPI 3.1の定義から生成しています。',
      h1: ['APIリファレンス。', '全エンドポイントと全フィールド。'],
      lede: 'APIが配信しているものと同じOpenAPI 3.1の定義から生成しているため、このページが実装とずれることはありません。',
      spec: 'OpenAPI 3.1 定義',
      interactive: 'インタラクティブ版',
      params: 'クエリパラメータ',
      required: '必須',
      optional: '任意',
      responses: 'レスポンス',
      eg: (v) => `例: ${v}`,
      arrayOf: (t) => `${t} の配列`,
      schemas: 'レスポンスのフィールド',
      errors: 'エラー',
      noParams: 'パラメータはありません。',
      limitsH2: 'レート制限',
      limits: [
        'APIキーも、呼び出し元ごとの利用枠もありません。ループに陥ったクライアントを止めるためのエッジ側のルールが1つあるだけです。`/v1/weather/` 配下の全パスを対象に、IPアドレスとCloudflareの拠点ごとに `10` 秒あたり `50` リクエストまで。',
        '超えるとエッジが `429` を返し、10秒の枠が空くまで待てば解除されます。それ以上の遮断はありません。カウントはキャッシュ参照の前に行うため、エッジから返るレスポンスもオリジンに届くリクエストと同じく1回と数えます。多数の観測所へ同時にリクエストする場合は、一度に50件を超えないようにしてください。なお、この数値は現時点の運用状況であり、保証ではありません。制限値もエンドポイントも予告なく変更される場合があり、本サービスは現状有姿で提供され、稼働率の保証はありません。',
      ],
    },

    colophon: {
      tagline: '日本の最寄り観測所の観測値を、JSONで配信。',
      facts: [
        ['10分', '更新間隔'],
        ['30日', '保持期間'],
        ['約1,300', '対象観測所'],
        ['気象庁 PDL 1.0', 'データ利用規約'],
      ],
      contact: 'お問い合わせ・ライセンス・削除依頼',
      security: '脆弱性のご報告',
      source: 'ソースコード（GitHub）',
    },

    footer: [
      '出典：気象庁ホームページ (',
      ') を加工して作成（編集責任：tenkinow） — 観測値は気象庁が公共データ利用規約（第1.0版）のもとで公開しているものを、発表形式そのままではなく正規化して提供しています。気象庁の公式プロダクトではなく、提供するのは観測値のみで予報は行いません。上流の配信は予告なく遅延・欠測・変更・停止することがあり、本サービスも古い値や不完全な値を返す、あるいは停止する場合があります。無保証での提供であり、防災など安全に関わる判断には利用しないでください。警報・予報は気象庁のサイトをご確認ください。',
    ],
  },
};

/**
 * Strings for a locale, defaulting to English.
 * @param {string} [lang]
 * @returns {typeof strings.en} — both locales share one shape.
 */
export const t = (lang) => strings[lang] || strings[defaultLocale];

/** The locale of the page currently rendered in the browser. */
export const pageLang = () =>
  typeof document === 'undefined' ? defaultLocale : document.documentElement.lang;
