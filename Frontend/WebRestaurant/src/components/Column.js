import React from "react";
import { Droppable } from "react-beautiful-dnd";
import Task from "./Task";

const Column = ({ column, tasks }) => {
  return (
    <div className="flex flex-col rounded-2xl overflow-hidden border border-gray-100 bg-gray-50/50 shadow-card">
      {/* Column Header */}
      <div
        className="px-5 py-4 flex items-center justify-between"
        style={{
          background: column.background,
        }}
      >
        <h3 className="text-sm font-bold text-white uppercase tracking-wider">
          {column.title}
        </h3>
        <span className="bg-white/20 text-white text-xs font-bold px-2.5 py-1 rounded-full">
          {tasks.length}
        </span>
      </div>

      {/* Tasks */}
      <Droppable droppableId={column.id} key={column.id}>
        {(provided, snapshot) => (
          <div
            ref={provided.innerRef}
            {...provided.droppableProps}
            className={`flex-1 p-3 space-y-3 min-h-[200px] max-h-[70vh] overflow-y-auto transition-colors duration-200 ${
              snapshot.isDraggingOver ? "bg-gray-100" : ""
            }`}
          >
            {tasks.map((task, index) => (
              <Task key={task.id} task={task} index={index} />
            ))}
            {provided.placeholder}
          </div>
        )}
      </Droppable>
    </div>
  );
};

export default Column;
