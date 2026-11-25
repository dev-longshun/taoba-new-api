/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import { dispatchSelectedPayment } from '../lib/payment'
import { parsePaymentMethods } from './use-topup-info'

describe('parsePaymentMethods', () => {
  test('keeps the provider so USDT methods reach the BEpusdt processor', async () => {
    // Shape returned by GET /api/user/topup/info for a BEpusdt method.
    const [method] = parsePaymentMethods(
      [
        {
          name: 'USDT-BEP20',
          type: 'usdt.bep20',
          icon: 'SiTether',
          min_topup: '10',
          provider: 'bepusdt',
        },
      ],
      0
    )

    expect(method).toMatchObject({
      type: 'usdt.bep20',
      provider: 'bepusdt',
      min_topup: 10,
    })

    const calls: string[] = []
    await dispatchSelectedPayment(method, 10, null, {
      regular: async (_amount, type) => {
        calls.push(`regular:${type}`)
        return true
      },
      bepusdt: async (_amount, type) => {
        calls.push(`bepusdt:${type}`)
        return true
      },
      waffo: async () => true,
      waffoPancake: async () => true,
    })

    expect(calls).toEqual(['bepusdt:usdt.bep20'])
  })

  test('leaves provider empty for epay methods', () => {
    const [method] = parsePaymentMethods(
      [{ name: 'Alipay', type: 'alipay' }],
      0
    )

    expect(method.provider).toBeUndefined()
  })
})
