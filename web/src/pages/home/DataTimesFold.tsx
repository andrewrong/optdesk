// The Data times fold shared by the home cards: shadcn Accordion around the text Python sends.
import { Clock } from 'lucide-react'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'

export function DataTimesFold({ text }: { text: string }) {
  return (
    <Accordion type="single" collapsible data-data-times>
      <AccordionItem value="data-times">
        <AccordionTrigger>
          <span className="flex items-center gap-1">
            <Clock aria-hidden className="size-4" />
            Data times
          </span>
        </AccordionTrigger>
        <AccordionContent>{text}</AccordionContent>
      </AccordionItem>
    </Accordion>
  )
}
