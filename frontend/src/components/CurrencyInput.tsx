'use client';

import { useState, useRef, useCallback } from 'react';

/**
 * A text input that displays dollar values with comma formatting.
 * When focused, shows the raw number for easy editing.
 * No spinner arrows.
 */
export default function CurrencyInput({
  value,
  onChange,
  className = '',
  required = false,
  placeholder = '0.00',
}: {
  value: number;
  onChange: (val: number) => void;
  className?: string;
  required?: boolean;
  placeholder?: string;
}) {
  const [focused, setFocused] = useState(false);
  const [rawText, setRawText] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  const formatDisplay = (n: number): string => {
    if (n === 0) return '';
    return n.toLocaleString('en-US', {
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
    });
  };

  const handleFocus = useCallback(() => {
    setFocused(true);
    // Show raw number (no commas) for editing
    setRawText(value === 0 ? '' : String(value));
  }, [value]);

  const handleBlur = useCallback(() => {
    setFocused(false);
    // Parse whatever was typed and commit
    const stripped = rawText.replace(/,/g, '');
    const parsed = parseFloat(stripped);
    if (!isNaN(parsed)) {
      onChange(Math.round(parsed * 100) / 100);
    } else if (rawText === '') {
      onChange(0);
    }
  }, [rawText, onChange]);

  const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const text = e.target.value;
    // Allow digits, decimal, minus, commas while typing
    if (/^-?[\d,]*\.?\d{0,2}$/.test(text) || text === '') {
      setRawText(text);
      // Live-update the value for controlled forms
      const stripped = text.replace(/,/g, '');
      const parsed = parseFloat(stripped);
      if (!isNaN(parsed)) {
        onChange(Math.round(parsed * 100) / 100);
      } else if (text === '') {
        onChange(0);
      }
    }
  }, [onChange]);

  return (
    <input
      ref={inputRef}
      type="text"
      inputMode="decimal"
      required={required}
      className={className}
      placeholder={placeholder}
      value={focused ? rawText : formatDisplay(value)}
      onFocus={handleFocus}
      onBlur={handleBlur}
      onChange={handleChange}
      style={{ /* hide any residual spinner */ MozAppearance: 'textfield' } as React.CSSProperties}
    />
  );
}
