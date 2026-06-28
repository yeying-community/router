import React from 'react';
import { Select } from 'antd';

const normalizeOptions = (options) =>
  (Array.isArray(options) ? options : []).map((option) => ({
    label: option?.label ?? option?.text ?? option?.name ?? option?.value,
    value: option?.value,
    disabled: option?.disabled === true,
  }));

const normalizeLabelInValueItem = (rawValue, normalizedOptions) => {
  if (rawValue == null || rawValue === '') {
    return undefined;
  }
  if (typeof rawValue === 'object' && !Array.isArray(rawValue)) {
    const nextValue = rawValue?.value;
    const matchedOption = normalizedOptions.find(
      (item) => String(item?.value ?? '') === String(nextValue ?? ''),
    );
    if (!matchedOption) {
      return rawValue;
    }
    return {
      ...rawValue,
      label: rawValue?.label ?? matchedOption.label,
      value: matchedOption.value,
    };
  }
  const matchedOption = normalizedOptions.find(
    (item) => String(item?.value ?? '') === String(rawValue ?? ''),
  );
  if (!matchedOption) {
    return {
      value: rawValue,
      label: String(rawValue),
    };
  }
  return {
    value: matchedOption.value,
    label: matchedOption.label,
  };
};

// Keep Select controlled values label-aware so async option sources do not
// regress to showing raw IDs after the popup closes or options refresh.
const normalizeControlledValue = (rawValue, normalizedOptions, multiple) => {
  if (multiple === true) {
    return (Array.isArray(rawValue) ? rawValue : [])
      .map((item) => normalizeLabelInValueItem(item, normalizedOptions))
      .filter(Boolean);
  }
  return normalizeLabelInValueItem(rawValue, normalizedOptions);
};

function AppSelect({
  className = '',
  options = [],
  value,
  placeholder,
  disabled,
  name,
  search,
  clearable,
  multiple,
  labelInValue,
  fluid = false,
  compact = false,
  variant,
  bordered,
  noResultsMessage,
  optionLabelProp,
  optionFilterProp,
  onChange,
  ...props
}) {
  const nextClassName = ['router-ui-select', fluid ? 'fluid' : '', className]
    .filter(Boolean)
    .join(' ');
  const mode = multiple === true ? 'multiple' : undefined;
  const normalizedOptions = normalizeOptions(options);
  const normalizedValue = normalizeControlledValue(value, normalizedOptions, multiple);
  const size = compact ? 'small' : undefined;
  const nextVariant =
    variant || (bordered === false ? 'borderless' : undefined);

  const filterOption =
    typeof search === 'function'
      ? (inputValue, option) =>
          search(
            normalizedOptions.map((item) => ({
              key: item.value,
              value: item.value,
              text: item.label,
            })),
            inputValue,
          ).some((item) => item?.value === option?.value)
      : undefined;

  const handleChange = (nextValue, option) => {
    const exposedValue =
      labelInValue === true
        ? nextValue
        : multiple === true
          ? (Array.isArray(nextValue)
              ? nextValue.map((item) => item?.value ?? item).filter((item) => item != null && item !== '')
              : [])
          : nextValue?.value ?? nextValue;
    if (typeof onChange === 'function') {
      onChange(null, {
        name,
        value: exposedValue,
        option,
      });
    }
  };

  return (
    <Select
      {...props}
      className={nextClassName}
      options={normalizedOptions}
      value={normalizedValue}
      placeholder={placeholder}
      disabled={disabled}
      showSearch={search === true || typeof search === 'function'}
      filterOption={filterOption}
      optionLabelProp={optionLabelProp || 'label'}
      optionFilterProp={optionFilterProp || 'label'}
      labelInValue
      allowClear={clearable === true}
      mode={mode}
      size={size}
      variant={nextVariant}
      notFoundContent={noResultsMessage}
      onChange={handleChange}
    />
  );
}

export default AppSelect;
