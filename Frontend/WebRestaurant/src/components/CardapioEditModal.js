import React, { useEffect, useState } from "react";
import Modal from "react-modal";
import MenuLayout from "./Menu";
import api from "../services/api";
import { useAuth } from "../context/AuthContext";
import Strings from "../constants/Strings";
import { toast } from "react-toastify";
import Texts from "../constants/Texts";
import helper from "../helpers/helper";
import ModalAddItens from "./ModalAddItens";
import productsModel from "../services/products.model";
import { FiX, FiSave, FiTrash2 } from "react-icons/fi";

const CardapioEditModal = ({
  isOpen,
  onClose,
  item,
  onSave,
  onRefreshItens,
}) => {
  const [formData, setFormData] = useState(Strings.initial_order(item));
  const { getUser } = useAuth();
  const [isOpenModal, setIsOpenModal] = useState(false);
  const [isCategory, setIsCategory] = useState(false);

  useEffect(() => {
    if (item) {
      setFormData(Strings.initial_order(item));
    }
  }, [item]);

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData({ ...formData, [name]: value });
  };

  const handleChangeMoney = (e) => {
    const { name, value } = e.target;
    const moneyPattern = /^\d+(\.\d{0,2})?$/;
    if (moneyPattern.test(value) || value === "") {
      setFormData({ ...formData, [name]: value });
    }
  };

  const openAlert = () => {
    toast.info(Texts.salve_primeiro);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    const body = {
      ...formData,
      Price: parseFloat(formData.Price),
      Id: parseInt(formData.ID) || null,
      ID: parseInt(formData.ID) || null,
      EstablishmentId: getUser().id,
      Categories: null,
    };

    try {
      if (formData.ID) {
        await api.put(`/products/update/${formData.ID}`, body);
      } else {
        await api.post("/products/create", body);
      }
      onSave(body);
      onRefreshItens();
      toast.success(Texts.cardapio_sucess);
      onClose();
    } catch (error) {
      toast.error(Texts.erro_cardapio);
    }
  };

  const deleteProduct = async () => {
    const resp = await productsModel.deleteProduct(item.ID);
    if (resp) {
      toast.success(Texts.removido_produto);
      onRefreshItens();
      onClose();
    } else {
      toast.error(Texts.falha_remover_produto);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onRequestClose={onClose}
      className="animate-fade-in"
      overlayClassName="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      style={{ content: { outline: "none" } }}
    >
      <div
        className="bg-white rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-modal animate-slide-up"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 className="text-lg font-bold text-gray-900">
            {item?.ID ? Texts.editar_itens : Texts.novo_produto}
          </h2>
          <button
            onClick={onClose}
            className="p-2 rounded-xl hover:bg-gray-100 transition-colors"
          >
            <FiX className="h-5 w-5 text-gray-500" />
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit} className="overflow-y-auto max-h-[70vh] p-6">
          <div className="flex gap-6 mb-6">
            {formData.Image && (
              <div className="flex-shrink-0">
                <img
                  src={formData.Image}
                  alt="Produto"
                  className="w-32 h-32 rounded-xl object-cover border-2 border-gray-100"
                />
              </div>
            )}
            <div className="flex-1 space-y-4">
              <div className="flex gap-4">
                {formData.ID && (
                  <div className="w-24">
                    <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                      {Texts.id}
                    </label>
                    <input
                      id="ID"
                      name="ID"
                      value={formData.ID}
                      disabled
                      className="block w-full px-3 py-2.5 bg-gray-100 border border-gray-200 rounded-xl text-sm text-gray-500"
                    />
                  </div>
                )}
                <div className="flex-1">
                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                    {Texts.nome}
                  </label>
                  <input
                    type="text"
                    id="Name"
                    required
                    maxLength={100}
                    name="Name"
                    value={formData.Name}
                    onChange={handleChange}
                    className="block w-full px-3 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white"
                    placeholder="Nome do produto"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
                  {Texts.preco}
                </label>
                <input
                  type="number"
                  id="Price"
                  name="Price"
                  value={formData.Price}
                  onChange={handleChangeMoney}
                  className="block w-full px-3 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white"
                  placeholder="0.00"
                />
              </div>
            </div>
          </div>

          <div className="mb-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
              Imagem URL
            </label>
            <input
              type="text"
              id="Image"
              maxLength={450}
              name="Image"
              value={formData.Image}
              onChange={handleChange}
              placeholder="https://..."
              className="block w-full px-3 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white"
            />
          </div>

          <div className="mb-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
              {Texts.description}
            </label>
            <textarea
              id="Description"
              name="Description"
              maxLength={150}
              value={formData.Description}
              onChange={handleChange}
              rows={3}
              className="block w-full px-3 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white resize-none"
              placeholder="Descrição do produto..."
            />
          </div>

          {/* Categories */}
          <div className="mb-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
              {Texts.categorias}
            </label>
            <div className="flex flex-wrap gap-2">
              {formData.Categories.map((e, i) => (
                <span
                  key={i}
                  className="inline-flex items-center px-3 py-1.5 rounded-full text-sm font-medium"
                  style={{ background: "#FEF2F2", color: "#EA1D2C" }}
                >
                  {e.Name}
                </span>
              ))}
              <button
                type="button"
                onClick={() => {
                  if (!item?.ID) openAlert();
                  else {
                    setIsCategory(true);
                    setIsOpenModal(true);
                  }
                }}
                className="inline-flex items-center px-3 py-1.5 rounded-full text-sm font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors"
              >
                + Adicionar
              </button>
            </div>
          </div>

          {/* Additionals */}
          <div className="mb-6">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
              {Texts.additional}
            </label>
            <div className="flex flex-wrap gap-2">
              {formData.Additional.map((e, i) => (
                <span
                  key={i}
                  className="inline-flex items-center px-3 py-1.5 rounded-full text-sm font-medium bg-gray-100 text-gray-700"
                >
                  {e.Name}
                  <span className="ml-1 text-xs text-gray-500">
                    ({helper.formatCurrency(e.Price)})
                  </span>
                </span>
              ))}
              <button
                type="button"
                onClick={() => {
                  if (!item?.ID) openAlert();
                  else {
                    setIsCategory(false);
                    setIsOpenModal(true);
                  }
                }}
                className="inline-flex items-center px-3 py-1.5 rounded-full text-sm font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors"
              >
                + Adicionar
              </button>
            </div>
          </div>
        </form>

        {/* Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-gray-100 bg-gray-50/50">
          <button
            type="button"
            onClick={() => deleteProduct()}
            disabled={!item?.ID}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <FiTrash2 className="h-4 w-4" />
            {Texts.remover_produto}
          </button>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-5 py-2.5 rounded-xl text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50 transition-colors"
            >
              {Texts.cancelar}
            </button>
            <button
              type="submit"
              onClick={handleSubmit}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all duration-200 hover:shadow-lg"
              style={{
                background: "linear-gradient(135deg, #EA1D2C, #C41420)",
              }}
            >
              <FiSave className="h-4 w-4" />
              {Texts.salvar}
            </button>
          </div>
        </div>
      </div>

      <ModalAddItens
        onClose={() => setIsOpenModal(false)}
        isOpen={isOpenModal}
        onSave={onSave}
        onRefreshItens={onRefreshItens}
        item={item}
        isCategory={isCategory}
      />
    </Modal>
  );
};

export default CardapioEditModal;
