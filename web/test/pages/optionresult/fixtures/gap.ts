import type { OptionResultPage } from '../../../../src/generated/option_result_page.v1'

export const gap = {
  schema: 'option_result_page',
  schema_version: 1,
  run_id: 'run-gap',
  build_id: 'build-gap',
  scope: 'scope-a',
  generated_at: '2026-01-02T09:30:00+08:00',
  data_times: [
    {
      source: 'src-a',
      data_time: '2026-01-02T09:00:00+08:00'
    }
  ],
  run_status: 'ok',
  title: 'Result title gap',
  stale: null,
  nothing: 'Nothing to show for this run',
  sections: [
    {
      key: 'sec-g',
      heading: 'Heading g',
      lead: 'Lead line g',
      scope: 'Scope text g',
      figures: [
        {
          label: 'Figure label g0',
          value: 'Figure value g0'
        },
        {
          label: 'Figure label g1',
          value: 'Figure value g1'
        },
        {
          label: 'Figure label g2',
          value: 'Figure value g2'
        },
        {
          label: 'Figure label g3',
          value: 'Figure value g3'
        },
        {
          label: 'Figure label g4',
          value: 'Figure value g4'
        },
        {
          label: 'Figure label g5',
          value: 'Figure value g5'
        }
      ],
      figure_notes: [
        'Figure note g'
      ],
      months: {
        title: 'Months title g',
        points: [],
        summary: null,
        empty: 'No months g'
      },
      running: {
        title: 'Running title g',
        points: [],
        summary: null,
        empty: 'No running g'
      },
      endings_title: 'Endings title g',
      endings_lines: [
        'Endings line g'
      ],
      endings: {
        title: 'Endings chart g',
        points: [],
        summary: null,
        empty: 'No endings g'
      },
      assignment: null,
      protection: null,
      books_title: 'Books title g',
      books_note: 'Books note g',
      books_lines: [
        'Books line g1',
        'Books line g2'
      ]
    }
  ],
  reliability: [
    {
      key: 'rel-gap',
      heading: 'Reliability heading gap',
      lines: [
        'Reliability line gap0',
        'Reliability line gap1',
        'Reliability line gap2'
      ]
    }
  ],
  facts: [
    {
      label: 'Fact label gap0',
      value: 'Fact value gap0'
    },
    {
      label: 'Fact label gap1',
      value: 'Fact value gap1'
    }
  ],
  notes: [
    'Note text gap1',
    'Note text gap2'
  ]
} satisfies OptionResultPage
