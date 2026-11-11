import type { OptionResultPage } from '../../../../src/generated/option_result_page.v1'

export const ok = {
  schema: 'option_result_page',
  schema_version: 1,
  run_id: 'run-ok',
  build_id: 'build-ok',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [
    {
      source: 'src-a',
      data_time: '2026-01-02T09:00:00+08:00'
    }
  ],
  run_status: 'ok',
  title: 'Result title ok',
  stale: null,
  nothing: null,
  sections: [
    {
      key: 'sec-a',
      heading: 'Heading a',
      lead: 'Lead line a',
      scope: 'Scope text a',
      figures: [
        {
          label: 'Figure label a0',
          value: 'Figure value a0'
        },
        {
          label: 'Figure label a1',
          value: 'Figure value a1'
        },
        {
          label: 'Figure label a2',
          value: 'Figure value a2'
        },
        {
          label: 'Figure label a3',
          value: 'Figure value a3'
        },
        {
          label: 'Figure label a4',
          value: 'Figure value a4'
        },
        {
          label: 'Figure label a5',
          value: 'Figure value a5'
        }
      ],
      figure_notes: [
        'Figure note a'
      ],
      months: {
        title: 'Months title a',
        points: [
          {
            label: 'M1',
            value: 1,
            text: 'Month text a1',
            muted: false
          },
          {
            label: 'M2',
            value: 2,
            text: 'Month text a2',
            muted: true
          }
        ],
        summary: 'M1 Month text a1; M2 Month text a2',
        empty: null
      },
      running: {
        title: 'Running title a',
        points: [
          {
            label: 'R1',
            value: 3,
            text: 'Running text a1',
            muted: false
          }
        ],
        summary: 'R1 Running text a1',
        empty: null
      },
      endings_title: 'Endings title a',
      endings_lines: [
        'Endings line a'
      ],
      endings: {
        title: 'Endings chart a',
        points: [
          {
            label: 'E1',
            value: 4,
            text: 'Ending text a1',
            muted: false
          }
        ],
        summary: 'E1 Ending text a1',
        empty: null
      },
      assignment: 'Assignment text a',
      protection: 'Protection text a',
      books_title: 'Books title a',
      books_note: 'Books note a',
      books_lines: [
        'Books line a1',
        'Books line a2'
      ]
    },
    {
      key: 'sec-b',
      heading: 'Heading b',
      lead: 'Lead line b',
      scope: 'Scope text b',
      figures: [
        {
          label: 'Figure label b0',
          value: 'Figure value b0'
        },
        {
          label: 'Figure label b1',
          value: 'Figure value b1'
        },
        {
          label: 'Figure label b2',
          value: 'Figure value b2'
        },
        {
          label: 'Figure label b3',
          value: 'Figure value b3'
        },
        {
          label: 'Figure label b4',
          value: 'Figure value b4'
        },
        {
          label: 'Figure label b5',
          value: 'Figure value b5'
        }
      ],
      figure_notes: [
        'Figure note b'
      ],
      months: {
        title: 'Months title b',
        points: [
          {
            label: 'M1',
            value: 1,
            text: 'Month text b1',
            muted: false
          },
          {
            label: 'M2',
            value: 2,
            text: 'Month text b2',
            muted: true
          }
        ],
        summary: 'M1 Month text b1; M2 Month text b2',
        empty: null
      },
      running: {
        title: 'Running title b',
        points: [
          {
            label: 'R1',
            value: 3,
            text: 'Running text b1',
            muted: false
          }
        ],
        summary: 'R1 Running text b1',
        empty: null
      },
      endings_title: 'Endings title b',
      endings_lines: [
        'Endings line b'
      ],
      endings: {
        title: 'Endings chart b',
        points: [
          {
            label: 'E1',
            value: 4,
            text: 'Ending text b1',
            muted: false
          }
        ],
        summary: 'E1 Ending text b1',
        empty: null
      },
      assignment: 'Assignment text b',
      protection: 'Protection text b',
      books_title: 'Books title b',
      books_note: 'Books note b',
      books_lines: [
        'Books line b1',
        'Books line b2'
      ]
    }
  ],
  reliability: [
    {
      key: 'rel-ok',
      heading: 'Reliability heading ok',
      lines: [
        'Reliability line ok0',
        'Reliability line ok1',
        'Reliability line ok2'
      ]
    }
  ],
  facts: [
    {
      label: 'Fact label ok0',
      value: 'Fact value ok0'
    },
    {
      label: 'Fact label ok1',
      value: 'Fact value ok1'
    }
  ],
  notes: [
    'Note text ok1',
    'Note text ok2'
  ]
} satisfies OptionResultPage
