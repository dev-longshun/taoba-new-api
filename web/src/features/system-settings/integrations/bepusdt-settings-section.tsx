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
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const BEPUSDT_MIN_TIMEOUT = 180

function isPayMethodList(value: string): boolean {
  try {
    const parsed: unknown = JSON.parse(value)
    return (
      Array.isArray(parsed) &&
      parsed.every(
        (item) =>
          typeof item === 'object' &&
          item !== null &&
          typeof item.name === 'string' &&
          item.name.trim() !== '' &&
          typeof item.type === 'string' &&
          item.type.trim() !== ''
      )
    )
  } catch {
    return false
  }
}

const schema = z.object({
  BEpusdtAddress: z
    .string()
    .trim()
    .refine((value) => value === '' || /^https?:\/\/\S+$/i.test(value), {
      message: 'Enter a URL starting with http:// or https://',
    }),
  BEpusdtToken: z.string(),
  BEpusdtTimeout: z.coerce.number().int().min(BEPUSDT_MIN_TIMEOUT),
  BEpusdtPayMethods: z.string().refine(isPayMethodList, {
    message: 'Enter a JSON array where every method has a name and a type',
  }),
})

type Values = z.infer<typeof schema>

interface BEpusdtSettingsSectionProps {
  defaultValues: Omit<Values, 'BEpusdtToken'>
}

export function BEpusdtSettingsSection(props: BEpusdtSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: { ...props.defaultValues, BEpusdtToken: '' },
  })

  const { isDirty, isSubmitting } = form.formState

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []
    const address = values.BEpusdtAddress.trim().replace(/\/+$/, '')
    if (address !== props.defaultValues.BEpusdtAddress) {
      updates.push({ key: 'BEpusdtAddress', value: address })
    }
    // The stored token is never sent to the browser; blank keeps it.
    if (values.BEpusdtToken.trim() !== '') {
      updates.push({ key: 'BEpusdtToken', value: values.BEpusdtToken.trim() })
    }
    if (values.BEpusdtTimeout !== props.defaultValues.BEpusdtTimeout) {
      updates.push({
        key: 'BEpusdtTimeout',
        value: String(values.BEpusdtTimeout),
      })
    }
    if (values.BEpusdtPayMethods !== props.defaultValues.BEpusdtPayMethods) {
      updates.push({
        key: 'BEpusdtPayMethods',
        value: values.BEpusdtPayMethods,
      })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset({ ...values, BEpusdtAddress: address, BEpusdtToken: '' })
  }

  return (
    <SettingsSection title={t('USDT Payment (BEpusdt)')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save USDT payment settings'
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'USDT top-ups are available when the address, token and at least one payment method are set. Orders use the top-up price, so the payment amount is converted to CNY and then to USDT by BEpusdt.'
            )}
          </p>

          <div className='grid gap-6 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='BEpusdtAddress'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('BEpusdt address')}</FormLabel>
                  <FormControl>
                    <Input placeholder='https://pay.example.com' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Base URL of your BEpusdt service')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='BEpusdtToken'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('BEpusdt API token')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave blank unless updating')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='BEpusdtTimeout'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Order timeout (seconds)')}</FormLabel>
                  <FormControl>
                    <Input type='number' min={BEPUSDT_MIN_TIMEOUT} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('How long a USDT order waits for the transfer')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='BEpusdtPayMethods'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('USDT payment methods')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={10}
                    className='font-mono text-xs'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'JSON array. type is the BEpusdt trade type such as usdt.trc20; min_topup is the minimum top-up amount for that network.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
