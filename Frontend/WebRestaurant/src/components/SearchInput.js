import React, { useState } from "react";
import { FiSearch } from "react-icons/fi";

const SearchInput = ({ onSearch }) => {
  const [searchTerm, setSearchTerm] = useState("");

  const handleChange = (event) => {
    setSearchTerm(event.target.value);
    onSearch(event.target.value);
  };

  return (
    <div className="relative flex-1">
      <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
        <FiSearch className="h-4 w-4 text-gray-400" />
      </div>
      <input
        type="text"
        placeholder="Buscar produto..."
        value={searchTerm}
        maxLength={100}
        onChange={handleChange}
        className="block w-full pl-10 pr-4 py-2.5 border border-gray-200 rounded-xl text-sm bg-white placeholder-gray-400 focus:bg-white focus:border-red-300 transition-colors"
      />
    </div>
  );
};

export default SearchInput;
