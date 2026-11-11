import type { ReportPage } from '../../../../src/generated/report_page.v2'

// Synthetic report_page fixture, close slot, written from contracts/schemas/page.report_page.v2.json alone.
export const okClose = {
  schema: 'report_page',
  schema_version: 2,
  market: 'US',
  slot: 'close',
  session_date: '2026-10-23',
  generated_at: '2026-10-23T21:30:00+08:00',
  title: 'Close report title',
  say: 'Close one-line verdict',
  banner: { left: 'Banner left text', right: 'Banner right text' },
  chips: [{ text: 'Chip A' }, { text: 'Chip B' }],
  status_lines: [
    { label: 'Status label one', text: 'Status content one' },
    { label: 'Status label two', text: 'Status content two' },
  ],
  blocks: {
    system_status: {
      key: 'system_status',
      title: 'Run overview title',
      sub: 'System status',
      say: 'Run overview verdict',
      state: 'ok',
      gaps: [],
      data_times: 'System status data time 2026-10-23 16:15',
      collapsed: false,
      lines: [{ label: 'System row label', text: 'System row text' }],
    },
    actions: {
      key: 'actions',
      title: 'Actions title',
      sub: 'Actions subtitle',
      say: 'Actions verdict',
      note: 'Actions note',
      state: 'ok',
      gaps: [],
      data_times: 'Actions data time 2026-10-23 16:00',
      link: { href: '/ledger', label: 'Actions link' },
      trades: {
        columns: ['Time', 'Account', 'Action', 'Underlying', 'Quantity', 'Price'],
        rows: [
          {
            time: { text: '09:45', tone: 'none' },
            account: { text: 'Account A', tone: 'none' },
            action: { text: 'Sell', tone: 'down' },
            instrument: { text: 'Demo underlying P 120', tone: 'none' },
            quantity: { text: '2', tone: 'none', value: 2 },
            price: { text: '1.20', tone: 'none', value: 1.2 },
          },
        ],
        footnote: 'Volume footnote text',
      },
      candidates: {
        heading: 'Candidates title text',
        groups: [
          {
            label: 'Candidate group A',
            items: [
              {
                symbol: 'DEMO',
                structure: 'Sell put',
                verdict: 'Consider',
                line1: 'Candidate row one',
                line2: 'Candidate row two',
                liquidity: 'Good liquidity',
              },
            ],
          },
        ],
      },
    },
    account_moves: {
      key: 'account_moves',
      title: 'Account change title',
      sub: 'Account change subtitle',
      say: 'Account change verdict',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: 'Account data time 2026-10-23 16:05',
      link: { href: '/household', label: 'Account swing' },
      accounts: {
        columns: ['Account', 'Net value', 'Amount today', 'Percent today', 'Max move'],
        rows: [
          {
            account: { text: 'Account A', tone: 'none' },
            net: { text: '$10,000', tone: 'none', value: 10000 },
            day_amount: { text: '+$120', tone: 'up', value: 120 },
            day_pct: { text: '+1.2%', tone: 'up', value: 1.2 },
            biggest: { text: 'DEMO +$80', tone: 'up' },
          },
        ],
      },
      movers: {
        heading: 'Change details title',
        columns: ['Underlying', 'Account', 'Amount today', 'Note'],
        rows: [
          {
            symbol: { text: 'DEMO', tone: 'none' },
            account: { text: 'Account A', tone: 'none' },
            day_amount: { text: '+$80', tone: 'up', value: 80 },
            note: { text: 'Change note text', tone: 'none' },
          },
        ],
      },
      meters: [
        {
          account: 'Account A',
          cash_after_assignment: { text: 'Cash after assignment $5,000', tone: 'none' },
          margin_buffer: { text: 'Margin buffer 40%', tone: 'flat' },
        },
      ],
    },
    risks: {
      key: 'risks',
      title: 'Risk section title',
      sub: 'Risk section subtitle',
      say: 'Risk section verdict',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: 'Risk data time 2026-10-23 16:10',
      link: { href: '/positions', label: 'Risk point' },
      items: [
        { kind: 'concentration', icon_key: 'pie', level: 'n', title: 'Risk item A', detail: 'Risk detail A' },
        { kind: 'expiry', icon_key: 'clock', level: 'w', title: 'Risk item B', detail: 'Risk detail B' },
        { kind: 'margin', icon_key: 'alert', level: 'd', title: 'Risk item C', detail: 'Risk detail C' },
      ],
    },
    market_watch: {
      key: 'market_watch',
      title: 'Market watch title',
      sub: 'Market watch subtitle',
      say: 'Market watch verdict',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: 'Market data time 2026-10-23 16:00',
      muted_note: 'Watch muted note',
      indices: {
        columns: ['Index', 'Close', 'Today', 'Five days'],
        rows: [
          {
            name: { text: 'Demo index', tone: 'none' },
            close: { text: '5,000', tone: 'none', value: 5000 },
            day: { text: '+0.5%', tone: 'up', value: 0.5 },
            five_day: { text: '-1.0%', tone: 'down', value: -1 },
            source_text: null,
          },
        ],
      },
      companies: {
        columns: ['Company', 'Today', 'News'],
        rows: [
          {
            symbol: { text: 'DEMO', tone: 'none' },
            day: { text: '+2.0%', tone: 'up', value: 2 },
            news: { text: 'Company news text', tone: 'none' },
          },
        ],
      },
    },
    global_assets: {
      key: 'global_assets',
      title: 'Global assets title',
      sub: 'Global assets subtitle',
      say: 'Global assets verdict',
      note: null,
      state: 'ok',
      gaps: [],
      data_times: 'Global data time 2026-10-23 16:00',
      chart_aria: 'Bar chart of the proxy changes of one day and five days of the global assets',
      proxies: [
        {
          symbol: 'AAA',
          label: 'Proxy A',
          day: { text: '+1.0%', tone: 'up', value: 1 },
          five_day: { text: '+2.5%', tone: 'up', value: 2.5 },
        },
        {
          symbol: 'BBB',
          label: 'Proxy B',
          day: { text: '-0.5%', tone: 'down', value: -0.5 },
          five_day: { text: '+0.3%', tone: 'up', value: 0.3 },
        },
      ],
      sector: {
        heading: 'Sector title text',
        columns: ['Sector', 'ETF', '1d', '5d', '20d'],
        rows: [
          {
            sector: { text: 'Tech', tone: 'none' },
            etf: { text: 'XLT', tone: 'none' },
            d1: { text: '+0.8%', tone: 'up', value: 0.8 },
            d5: { text: '+1.5%', tone: 'up', value: 1.5 },
            d20: { text: '-2.0%', tone: 'down', value: -2 },
          },
        ],
      },
      rotation: { title: 'Rotation title text', detail: 'Rotation detail text' },
    },
  },
} satisfies ReportPage
