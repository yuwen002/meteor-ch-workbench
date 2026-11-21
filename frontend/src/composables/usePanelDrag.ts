import { ref } from 'vue'
import { clampPanelPosition, dragPanelPosition, PANEL_MARGIN, type PanelPoint } from '../utils/dragPanel'

interface DragSession {
  pointerId: number
  startCursor: PanelPoint
  origin: PanelPoint
}

export function usePanelDrag(size: () => { width: number; height: number }) {
  const offset = ref<PanelPoint | null>(null)
  let session: DragSession | null = null

  function currentPosition(base: PanelPoint): PanelPoint {
    return offset.value ?? base
  }

  function startDrag(event: PointerEvent, base: PanelPoint) {
    if (event.button !== 0) return
    const origin = currentPosition(base)
    session = {
      pointerId: event.pointerId,
      startCursor: { x: event.clientX, y: event.clientY },
      origin,
    }
    const target = event.currentTarget as HTMLElement | null
    target?.setPointerCapture?.(event.pointerId)
    event.preventDefault()
  }

  function moveDrag(event: PointerEvent) {
    if (!session || event.pointerId !== session.pointerId) return
    offset.value = dragPanelPosition(
      session.origin,
      session.startCursor,
      { x: event.clientX, y: event.clientY },
      size(),
      { width: window.innerWidth, height: window.innerHeight },
      PANEL_MARGIN,
    )
  }

  function endDrag(event?: PointerEvent) {
    if (!session) return
    if (event && event.pointerId !== session.pointerId) return
    const target = event?.currentTarget as HTMLElement | null
    target?.releasePointerCapture?.(session.pointerId)
    session = null
  }

  function clampToViewport(base: PanelPoint) {
    offset.value = clampPanelPosition(
      currentPosition(base),
      size(),
      { width: window.innerWidth, height: window.innerHeight },
      PANEL_MARGIN,
    )
  }

  function resetDrag() {
    session = null
    offset.value = null
  }

  return { offset, currentPosition, startDrag, moveDrag, endDrag, clampToViewport, resetDrag }
}