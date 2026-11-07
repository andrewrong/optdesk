import type { OptionResultPage } from '../../../../src/generated/option_result_page.v1'

export const stale = {
  schema: 'option_result_page',
  schema_version: 1,
  run_id: 'run-stale',
  build_id: 'build-stale',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [
    {
      source: 'src-a',
      data_time: '2026-01-02T09:00:00+08:00'
    }
  ],
  run_status: 'ok',
  title: 'Result title stale',
  stale: {
    data_time: '2026-01-01T09:00:00+08:00',
    stale_after: '2026-01-01T10:00:00+08:00',
    text: 'Stale label text'
  },
  nothing: null,
  sections: [
    {
      key: 'sec-s',
      heading: 'Heading s',
      lead: 'Lead line s',
      scope: 'Scope text s',
      figures: [
        {
          label: 'Figure label s0',
          value: 'Figure value s0'
        },
        {
          label: 'Figure label s1',
          value: 'Figure value s1'
        },
        {
          label: 'Figure label s2',
          value: 'Figure value s2'
        },
        {
          label: 'Figure label s3',
          value: 'Figure value s3'
        },
        {
          label: 'Figure label s4',
          value: 'Figure value s4'
        },
        {
          label: 'Figure label s5',
          value: 'Figure value s5'
        }
      ],
      figure_notes: [
        'Figure note s'
      ],
      months: {
        title: 'Months title s',
        points: [
          {
            label: 'M1',
            value: 1,
            text: 'Month text s1',
            muted: false
          },
          {
            label: 'M2',
            value: 2,
            text: 'Month text s2',
            muted: true
          }
        ],
        summary: 'M1 Month text s1；M2 Month text s2',
        empty: null
      },
      running: {
        title: 'Running title s',
        points: [
          {
            label: 'R1',
            value: 3,
            text: 'Running text s1',
            muted: false
          }
        ],
        summary: 'R1 Running text s1',
        empty: null
      },
      endings_title: 'Endings title s',
      endings_lines: [
        'Endings line s'
      ],
      endings: {
        title: 'Endings chart s',
        points: [
          {
            label: 'E1',
            value: 4,
            text: 'Ending text s1',
            muted: false
          }
        ],
        summary: 'E1 Ending text s1',
        empty: null
      },
      assignment: 'Assignment text s',
      protection: 'Protection text s',
      books_title: 'Books title s',
      books_note: 'Books note s',
      books_lines: [
        'Books line s1',
        'Books line s2'
      ]
    }
  ],
  reliability: [
    {
      key: 'rel-stale',
      heading: 'Reliability heading stale',
      lines: [
        'Reliability line stale0',
        'Reliability line stale1',
        'Reliability line stale2'
      ]
    }
  ],
  facts: [
    {
      label: 'Fact label stale0',
      value: 'Fact value stale0'
    },
    {
      label: 'Fact label stale1',
      value: 'Fact value stale1'
    }
  ],
  notes: [
    'Note text stale1',
    'Note text stale2'
  ]
} satisfies OptionResultPage
