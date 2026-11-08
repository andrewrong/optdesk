import type { ReportPage } from '../../../../src/generated/report_page.v2'

// Synthetic report_page fixture, close slot, written from contracts/schemas/page.report_page.v2.json alone.
export const okClose = {
  schema: 'report_page',
  schema_version: 2,
  market: 'US',
  slot: 'close',
  session_date: '2026-10-23',
  generated_at: '2026-10-23T21:30:00+08:00',
  title: '收盘报告标题',
  say: '收盘一句话结论',
  banner: { left: '横幅左侧文字', right: '横幅右侧文字' },
  chips: [{ text: '筹码甲' }, { text: '筹码乙' }],
  status_lines: [
    { label: '状态标签一', text: '状态内容一' },
    { label: '状态标签二', text: '状态内容二' },
  ],
  blocks: {
    system_status: {
      key: 'system_status',
      title: '运行概况标题',
      sub: '系统状态',
      say: '运行概况结论',
      state: 'ok',
      gaps: [],
      data_times: '系统状态数据时间 2026-10-23 16:15',
      collapsed: false,
      lines: [{ label: '系统行标签', text: '系统行文字' }],
    },
    actions: {
      key: 'actions',
      title: '操作区标题',
      sub: '操作区副标题',
      say: '操作区结论',
      note: '操作区备注',
      state: 'ok',
      gaps: [],
      data_times: '操作区数据时间 2026-10-23 16:00',
      link: { href: '/ledger', label: '操作区链接' },
      trades: {
        columns: ['时间', '账户', '动作', '标的', '数量', '价格'],
        rows: [
          {
            time: { text: '09:45', tone: 'none' },
            account: { text: '账户甲', tone: 'none' },
            action: { text: '卖出', tone: 'down' },
            instrument: { text: '演示标的 P 120', tone: 'none' },
            quantity: { text: '2', tone: 'none', value: 2 },
            price: { text: '1.20', tone: 'none', value: 1.2 },
          },
        ],
        footnote: '成交脚注文字',
      },
      candidates: {
        heading: '候选标题文字',
        groups: [
          {
            label: '候选分组甲',
            items: [
              {
                symbol: 'DEMO',
                structure: '卖出认沽',
                verdict: '可考虑',
                line1: '候选第一行',
                line2: '候选第二行',
                liquidity: '流动性良好',
              },
            ],
          },
        ],
      },
    },
    account_moves: {
      key: 'account_moves',
      title: '账户变动标题',
      sub: '账户变动副标题',
      say: '账户变动结论',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: '账户数据时间 2026-10-23 16:05',
      link: { href: '/household', label: '账户波动' },
      accounts: {
        columns: ['账户', '净值', '当日金额', '当日百分比', '最大变动'],
        rows: [
          {
            account: { text: '账户甲', tone: 'none' },
            net: { text: '$10,000', tone: 'none', value: 10000 },
            day_amount: { text: '+$120', tone: 'up', value: 120 },
            day_pct: { text: '+1.2%', tone: 'up', value: 1.2 },
            biggest: { text: 'DEMO +$80', tone: 'up' },
          },
        ],
      },
      movers: {
        heading: '变动明细标题',
        columns: ['标的', '账户', '当日金额', '说明'],
        rows: [
          {
            symbol: { text: 'DEMO', tone: 'none' },
            account: { text: '账户甲', tone: 'none' },
            day_amount: { text: '+$80', tone: 'up', value: 80 },
            note: { text: '变动说明文字', tone: 'none' },
          },
        ],
      },
      meters: [
        {
          account: '账户甲',
          cash_after_assignment: { text: '指派后现金 $5,000', tone: 'none' },
          margin_buffer: { text: '保证金缓冲 40%', tone: 'flat' },
        },
      ],
    },
    risks: {
      key: 'risks',
      title: '风险区标题',
      sub: '风险区副标题',
      say: '风险区结论',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: '风险数据时间 2026-10-23 16:10',
      link: { href: '/positions', label: '风险点' },
      items: [
        { kind: 'concentration', icon_key: 'pie', level: 'n', title: '风险项甲', detail: '风险详情甲' },
        { kind: 'expiry', icon_key: 'clock', level: 'w', title: '风险项乙', detail: '风险详情乙' },
        { kind: 'margin', icon_key: 'alert', level: 'd', title: '风险项丙', detail: '风险详情丙' },
      ],
    },
    market_watch: {
      key: 'market_watch',
      title: '市场观察标题',
      sub: '市场观察副标题',
      say: '市场观察结论',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: '市场数据时间 2026-10-23 16:00',
      muted_note: '观察静音备注',
      indices: {
        columns: ['指数', '收盘', '当日', '五日'],
        rows: [
          {
            name: { text: '演示指数', tone: 'none' },
            close: { text: '5,000', tone: 'none', value: 5000 },
            day: { text: '+0.5%', tone: 'up', value: 0.5 },
            five_day: { text: '-1.0%', tone: 'down', value: -1 },
            source_text: null,
          },
        ],
      },
      companies: {
        columns: ['公司', '当日', '消息'],
        rows: [
          {
            symbol: { text: 'DEMO', tone: 'none' },
            day: { text: '+2.0%', tone: 'up', value: 2 },
            news: { text: '公司消息文字', tone: 'none' },
          },
        ],
      },
    },
    global_assets: {
      key: 'global_assets',
      title: '全球资产标题',
      sub: '全球资产副标题',
      say: '全球资产结论',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: '全球数据时间 2026-10-23 16:00',
      chart_aria: '全球资产代理当日与五日涨跌柱状图',
      proxies: [
        {
          symbol: 'AAA',
          label: '代理甲',
          day: { text: '+1.0%', tone: 'up', value: 1 },
          five_day: { text: '+2.5%', tone: 'up', value: 2.5 },
        },
        {
          symbol: 'BBB',
          label: '代理乙',
          day: { text: '-0.5%', tone: 'down', value: -0.5 },
          five_day: { text: '+0.3%', tone: 'up', value: 0.3 },
        },
      ],
      sector: {
        heading: '板块标题文字',
        columns: ['板块', 'ETF', '1日', '5日', '20日'],
        rows: [
          {
            sector: { text: '科技', tone: 'none' },
            etf: { text: 'XLT', tone: 'none' },
            d1: { text: '+0.8%', tone: 'up', value: 0.8 },
            d5: { text: '+1.5%', tone: 'up', value: 1.5 },
            d20: { text: '-2.0%', tone: 'down', value: -2 },
          },
        ],
      },
      rotation: { title: '轮动标题文字', detail: '轮动详情文字' },
    },
  },
} satisfies ReportPage
