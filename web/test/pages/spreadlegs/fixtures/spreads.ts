// A typed copy of contracts/fixtures/page.spread_legs_page.v1.ok.json: a put spread, an iron condor, a covered call with its gaps and an unidentified unit.
import type { SpreadLegsPage } from '@/generated/spread_legs_page.v1'

export const spreads: SpreadLegsPage = {
  "schema": "spread_legs_page",
  "schema_version": 1,
  "run_id": "run-demo-0001",
  "build_id": "build-demo-0001",
  "scope": "spread-legs",
  "generated_at": "2026-11-21T09:00:00+08:00",
  "data_times": [
    {
      "source": "ledger_replay",
      "data_time": "2026-11-20T00:00:00-05:00"
    }
  ],
  "run_status": "ok",
  "stale": {
    "data_time": "2026-11-20T00:00:00-05:00",
    "text": "数据日 2026-11-20",
    "is_stale": false
  },
  "title": "价差腿 Spread legs",
  "sentence": "共 3 个组合，另有 2 个单腿持仓不在此页（见“持仓结构”页）；按到期天数由近到远排列。",
  "note": "净收入 = 各腿（-数量 x 开仓价 x 合约乘数）之和，只含开仓价，不含手续费和平仓。",
  "groups": [
    {
      "title": "价差组合 Spreads",
      "structures": [
        {
          "symbol": "AAA",
          "href": "/options/structures/put_spread-aaa-2026-12-18",
          "kind_text": "认沽价差 Put spread",
          "kind_gloss": "卖出价格较高的认沽、买入价格较低的认沽",
          "expiry_text": "到期 2026-12-18（27 天）",
          "legs": [
            {
              "side_text": "卖出认沽 Short put",
              "strike": "95.00",
              "expiry_text": "2026-12-18",
              "quantity": "-2",
              "open_price": "2.40"
            },
            {
              "side_text": "买入认沽 Long put",
              "strike": "90.00",
              "expiry_text": "2026-12-18",
              "quantity": "+2",
              "open_price": "1.10"
            }
          ],
          "net_credit": {
            "kind": "value",
            "text": "+260.00 美元",
            "value": 260.0,
            "unit": "usd"
          },
          "max_loss": {
            "kind": "value",
            "text": "-740.00",
            "value": -740.0,
            "unit": "usd"
          },
          "max_gain": {
            "kind": "value",
            "text": "+260.00",
            "value": 260.0,
            "unit": "usd"
          },
          "break_even": {
            "kind": "values",
            "text": "93.70",
            "values": [
              93.7
            ]
          },
          "reason": null
        },
        {
          "symbol": "BBB",
          "href": "/options/structures/iron_condor-bbb-2026-12-18",
          "kind_text": "铁鹰 Iron condor",
          "kind_gloss": "一个认沽价差加一个认购价差",
          "expiry_text": "到期 2026-12-18（27 天）",
          "legs": [
            {
              "side_text": "卖出认沽 Short put",
              "strike": "90.00",
              "expiry_text": "2026-12-18",
              "quantity": "-1",
              "open_price": "2.00"
            },
            {
              "side_text": "买入认沽 Long put",
              "strike": "85.00",
              "expiry_text": "2026-12-18",
              "quantity": "+1",
              "open_price": "0.90"
            },
            {
              "side_text": "卖出认购 Short call",
              "strike": "110.00",
              "expiry_text": "2026-12-18",
              "quantity": "-1",
              "open_price": "2.10"
            },
            {
              "side_text": "买入认购 Long call",
              "strike": "115.00",
              "expiry_text": "2026-12-18",
              "quantity": "+1",
              "open_price": "0.80"
            }
          ],
          "net_credit": {
            "kind": "value",
            "text": "+310.00 美元",
            "value": 310.0,
            "unit": "usd"
          },
          "max_loss": {
            "kind": "value",
            "text": "-690.00",
            "value": -690.0,
            "unit": "usd"
          },
          "max_gain": {
            "kind": "value",
            "text": "+310.00",
            "value": 310.0,
            "unit": "usd"
          },
          "break_even": {
            "kind": "values",
            "text": "86.90 和 113.10",
            "values": [
              86.9,
              113.1
            ]
          },
          "reason": null
        },
        {
          "symbol": "CCC",
          "href": "/options/structures/covered_call-ccc-2026-12-18",
          "kind_text": "备兑认购 Covered call",
          "kind_gloss": "持有正股、卖出认购",
          "expiry_text": "到期 2026-12-18（27 天）",
          "legs": [
            {
              "side_text": "卖出认购 Short call",
              "strike": "120.00",
              "expiry_text": "2026-12-18",
              "quantity": "-1",
              "open_price": "1.80"
            }
          ],
          "net_credit": {
            "kind": "value",
            "text": "+180.00 美元",
            "value": 180.0,
            "unit": "usd"
          },
          "max_loss": {
            "kind": "gap",
            "text": "无上限规则（含股票，此页不算）",
            "reason": "no_rule"
          },
          "max_gain": {
            "kind": "value",
            "text": "+1,420.00",
            "value": 1420.0,
            "unit": "usd"
          },
          "break_even": {
            "kind": "gap",
            "text": "此页不算（含股票）",
            "values": [],
            "reason": "no_rule"
          },
          "reason": null
        }
      ]
    },
    {
      "title": "未识别组合 Unidentified",
      "structures": [
        {
          "symbol": "DDD",
          "href": "/options/structures/other-ddd-2026-12-18",
          "kind_text": "未识别组合 Unidentified",
          "kind_gloss": "匹配器未能命名的一组期权腿",
          "expiry_text": "到期 2026-12-18（27 天）",
          "legs": [
            {
              "side_text": "卖出认沽 Short put",
              "strike": "50.00",
              "expiry_text": "2026-12-18",
              "quantity": "-1",
              "open_price": "1.00"
            },
            {
              "side_text": "卖出认购 Short call",
              "strike": "80.00",
              "expiry_text": "2027-01-15",
              "quantity": "-3",
              "open_price": "0.50"
            }
          ],
          "net_credit": {
            "kind": "gap",
            "text": "此结构没有这项规则",
            "reason": "no_rule"
          },
          "max_loss": {
            "kind": "gap",
            "text": "此结构没有这项规则",
            "reason": "no_rule"
          },
          "max_gain": {
            "kind": "gap",
            "text": "此结构没有这项规则",
            "reason": "no_rule"
          },
          "break_even": {
            "kind": "gap",
            "text": "此结构没有这项规则",
            "values": [],
            "reason": "no_rule"
          },
          "reason": "匹配器未能命名这组腿（到期日不同、数量不成对）"
        }
      ]
    }
  ]
}
