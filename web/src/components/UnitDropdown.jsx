import React, { useMemo } from 'react';
import { Dropdown } from 'semantic-ui-react';

const VARIANT_CLASS_MAP = {
  section: 'router-section-dropdown',
  inline: 'router-inline-dropdown',
  header: 'router-table-header-dropdown',
  inputUnit: 'router-section-input-unit-dropdown',
};

const normalizeOptionItem = (item) => {
  if (item == null) {
    return null;
  }
  if (typeof item === 'string' || typeof item === 'number') {
    const value = `${item}`;
    return {
      key: value,
      value,
      text: value,
    };
  }
  const value =
    item.value ?? item.code ?? item.id ?? item.key ?? item.text ?? item.label ?? '';
  if (value === '') {
    return null;
  }
  const { label, ...rest } = item;
  return {
    ...rest,
    key: item.key ?? value,
    value,
    text: item.text ?? item.label ?? item.name ?? `${value}`,
  };
};

export default function UnitDropdown({
  variant = 'section',
  className = '',
  options = [],
  selection = true,
  selectOnBlur = false,
  ...rest
}) {
  const normalizedOptions = useMemo(
    () => (Array.isArray(options) ? options.map(normalizeOptionItem).filter(Boolean) : []),
    [options],
  );
  const variantClassName = VARIANT_CLASS_MAP[variant] || VARIANT_CLASS_MAP.section;
  const combinedClassName = [variantClassName, className].filter(Boolean).join(' ');

  return (
    <Dropdown
      selection={selection}
      selectOnBlur={selectOnBlur}
      className={combinedClassName}
      options={normalizedOptions}
      {...rest}
    />
  );
}
