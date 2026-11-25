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
import i18next from 'i18next'
import { useState, useCallback } from 'react'

import { handleServerError } from '@/lib/handle-server-error'

import { requestBEpusdtPayment, isApiSuccess } from '../api'
import type { BEpusdtOrderPayload } from '../types'

/**
 * Creates USDT orders through the BEpusdt gateway. A created order is kept in
 * `order` so the wallet page can show the transfer details.
 */
export function useBEpusdtPayment() {
  const [processing, setProcessing] = useState(false)
  const [order, setOrder] = useState<BEpusdtOrderPayload | null>(null)

  const processBEpusdtPayment = useCallback(
    async (topupAmount: number, tradeType: string) => {
      setProcessing(true)
      try {
        const response = await requestBEpusdtPayment({
          amount: Math.floor(topupAmount),
          payment_method: tradeType,
        })
        const usdt = isApiSuccess(response) ? response.data?.usdt : undefined
        if (usdt?.trade_no && usdt.address && usdt.amount) {
          setOrder(usdt)
          return true
        }

        handleServerError(response, i18next.t('Payment request failed'))
        return false
      } catch (error) {
        handleServerError(error, i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  const clearOrder = useCallback(() => setOrder(null), [])

  return { processing, order, processBEpusdtPayment, clearOrder }
}
