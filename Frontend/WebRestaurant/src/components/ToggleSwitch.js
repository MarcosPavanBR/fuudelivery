import React, { useEffect, useState } from "react";

const ToggleSwitch = ({ label, onChange, checked }) => {
  const [isChecked, setIsChecked] = useState(checked ?? false);

  const toggleChecked = () => {
    onChange(!isChecked);
    setIsChecked((prevState) => !prevState);
  };

  useEffect(() => {
    setIsChecked(checked);
  }, [checked]);

  return (
    <div className="flex items-center">
      <label htmlFor="toggle" className="flex items-center cursor-pointer">
        <div className="relative">
          <input
            type="checkbox"
            id="toggle"
            className="sr-only"
            checked={isChecked}
            onChange={toggleChecked}
          />
          <div
            className="w-10 h-5 rounded-full shadow-inner transition-colors duration-200"
            style={{
              background: isChecked
                ? "linear-gradient(135deg, #EA1D2C, #C41420)"
                : "#D1D5DB",
            }}
          />
          <div
            className={`absolute w-4 h-4 bg-white rounded-full shadow top-0.5 transition-transform duration-200 ${
              isChecked ? "translate-x-[22px]" : "translate-x-0.5"
            }`}
          />
        </div>
        {label && (
          <span className="ml-2 text-sm font-medium text-gray-700">{label}</span>
        )}
      </label>
    </div>
  );
};

export default ToggleSwitch;
