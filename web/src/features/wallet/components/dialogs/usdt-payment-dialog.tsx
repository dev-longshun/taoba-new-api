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
import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import { getBEpusdtOrderStatus } from '../../api'
import { BEPUSDT_NETWORK_LABELS, BEPUSDT_STATUS_POLL_MS } from '../../constants'
import { formatCurrency } from '../../lib'
import type { BEpusdtOrderPayload } from '../../types'

interface UsdtPaymentDialogProps {
  /** Order to pay; the dialog is open while it is set */
  order: BEpusdtOrderPayload | null
  onOpenChange: (open: boolean) => void
  /** Called once when the gateway confirms the payment */
  onPaid: () => void
}

interface CopyFieldProps {
  label: string
  value: string
}

function CopyField(props: CopyFieldProps) {
  return (
    <div className='space-y-1.5'>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      <div className='bg-muted flex items-center gap-2 rounded-lg p-3'>
        <code className='min-w-0 flex-1 font-mono text-sm break-all'>
          {props.value}
        </code>
        <CopyButton value={props.value} aria-label={props.label} />
      </div>
    </div>
  )
}

function formatCountdown(totalSeconds: number): string {
  const seconds = Math.max(0, totalSeconds)
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  return `${minutes}:${rest.toString().padStart(2, '0')}`
}

export function UsdtPaymentDialog(props: UsdtPaymentDialogProps) {
  const { t } = useTranslation()
  const order = props.order
  const tradeNo = order?.trade_no ?? ''
  const paidHandledRef = useRef('')
  const deadline = useMemo(
    () => (order ? Date.now() + order.expire_in * 1000 : 0),
    [order]
  )
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!order) return
    setNow(Date.now())
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [order])

  const secondsLeft = Math.round((deadline - now) / 1000)
  const countdownExpired = order !== null && secondsLeft <= 0
  const statusQuery = useQuery({
    queryKey: ['wallet', 'bepusdt-order-status', tradeNo],
    queryFn: () => getBEpusdtOrderStatus(tradeNo),
    enabled: order !== null && !countdownExpired,
    refetchInterval: BEPUSDT_STATUS_POLL_MS,
    meta: { errorToast: false },
  })
  const status = statusQuery.data?.data?.status
  const expired = countdownExpired || status === 'expired'

  const { onPaid, onOpenChange } = props
  useEffect(() => {
    if (status !== 'success' || paidHandledRef.current === tradeNo) return
    paidHandledRef.current = tradeNo
    toast.success(t('Payment received'))
    onPaid()
    onOpenChange(false)
  }, [status, tradeNo, onPaid, onOpenChange, t])

  if (!order) return null

  const network = BEPUSDT_NETWORK_LABELS[order.trade_type] ?? order.trade_type

  return (
    <Dialog
      open
      onOpenChange={props.onOpenChange}
      title={t('USDT payment')}
      description={t(
        'Transfer the exact amount to the address below on the {{network}} network',
        { network }
      )}
      contentClassName='sm:max-w-md'
      footer={
        <Button className='w-full' onClick={() => props.onOpenChange(false)}>
          {t('I have paid')}
        </Button>
      }
    >
      <div className='space-y-4'>
        {expired ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('This order has expired. Do not transfer to this address.')}
            </AlertDescription>
          </Alert>
        ) : null}

        <div className='flex flex-col items-center gap-2'>
          <div className='rounded-lg bg-white p-3'>
            <QRCodeSVG value={order.address} size={176} />
          </div>
          <p className='text-sm font-medium'>
            {t('Network')}: {network}
          </p>
        </div>

        <CopyField label={t('Amount (USDT)')} value={order.amount} />
        <CopyField label={t('Receiving address')} value={order.address} />

        <div className='space-y-1 text-center text-sm'>
          <p className='text-muted-foreground'>
            {t('Order amount')}: ¥{formatCurrency(order.money)}
          </p>
          {expired ? null : (
            <p className='font-medium tabular-nums'>
              {t('Time left: {{time}}', {
                time: formatCountdown(secondsLeft),
              })}
            </p>
          )}
          {expired ? null : (
            <p className='text-muted-foreground flex items-center justify-center gap-1.5 text-xs'>
              <Loader2 className='size-3 animate-spin' aria-hidden='true' />
              {t('Waiting for payment...')}
            </p>
          )}
        </div>

        <ul className='text-muted-foreground list-disc space-y-1 ps-4 text-xs'>
          <li>
            {t(
              'The received amount must match exactly. If your exchange deducts a withdrawal fee, add it so the amount received equals the amount above.'
            )}
          </li>
          <li>
            {t(
              'Transfers on another network, with a different amount, or after the order expires are not credited automatically. Contact support with the transaction hash.'
            )}
          </li>
          <li>
            {t(
              'Payment is confirmed automatically; you can close this window after paying'
            )}
          </li>
        </ul>
      </div>
    </Dialog>
  )
}
