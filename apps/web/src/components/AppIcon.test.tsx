import { render } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import AppIcon from './AppIcon'

describe('AppIcon', () => {
  it('renders an icon for a known category', () => {
    const { container } = render(<AppIcon category="storage" />)
    expect(container.querySelector('svg')).toBeTruthy()
  })

  it('falls back to a default icon for an unknown app id', () => {
    const { container } = render(<AppIcon appId="does-not-exist" />)
    expect(container.querySelector('svg')).toBeTruthy()
  })
})
