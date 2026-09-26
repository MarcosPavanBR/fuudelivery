import React, { useEffect, useState } from "react";
import Modal from "react-modal";
import api from "../services/api";
import { useAuth } from "../context/AuthContext";
import { toast } from "react-toastify";
import Texts from "../constants/Texts";
import helper from "../helpers/helper";
import ModalAddItens from "./ModalAddItens";
import productsModel from "../services/products.model";
import { FiX, FiSave, FiTrash2, FiCamera, FiLoader } from "react-icons/fi";
import { uploadImage } from "../helpers/imageUpload";
import { establishmentIdOf } from "../helpers/session";

const getInitialFormData = (item) => ({
  ID: item?.ID || "",
  Name: item?.Name || "",
  Description: item?.Description || "",
  Price: item?.Price || 0,
  Image: item?.Image || "",
  Categories: item?.Categories ?? [],
  Additional: item?.Additional ?? [],
});

const CardapioEditModal = ({
  isOpen,
  onClose,
  item,
  onSave,
  onRefreshItens,
}) => {
  const { getUser } = useAuth();
  const [formData, setFormData] = useState(getInitialFormData(item));
  const [isOpenModal, setIsOpenModal] = useState(false);
  const [isCategory, setIsCategory] = useState(false);
  const [uploading, setUploading] = useState(false);

  // Foto do produto por upload (antes era um campo para colar link). O
  // servidor exige o id do produto (/upload/products/:id confere o dono).
  const handleImage = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file || !formData.ID) return;
    setUploading(true);
    try {
      const url = await uploadImage(`products/${formData.ID}`, file, 1200);
      // Grava na hora (como a logo): a foto não depende do "Salvar".
      await api.put(`/products/update/${formData.ID}`, {
        name: formData.Name,
        description: formData.Description,
        price: parseFloat(formData.Price) || 0,
        image: url,
      });
      setFormData((prev) => ({ ...prev, Image: url }));
      onRefreshItens();
      toast.success("Foto do produto atualizada!");
    } catch (err) {
      toast.error(err.message || "Erro ao enviar a foto");
    }
    setUploading(false);
  };

  // Mesmo produto atualizado (vínculo de categoria/adicional): troca só as
  // listas e preserva o que está sendo editado. Produto diferente: recarrega.
  useEffect(() => {
    if (!item) return;
    setFormData((prev) =>
      prev.ID && prev.ID === item.ID
        ? { ...prev, Categories: item.Categories ?? [], Additional: item.Additional ?? [] }
        : getInitialFormData(item)
    );
  }, [item]);

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleChangeMoney = (e) => {
    const { name, value } = e.target;
    const moneyPattern = /^\d+(\.\d{0,2})?$/;
    if (moneyPattern.test(value) || value === "") {
      setFormData((prev) => ({ ...prev, [name]: value }));
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
      EstablishmentId: establishmentIdOf(getUser()),
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
    if (!window.confirm(`Remover "${formData.Name}" do cardápio? Isto não pode ser desfeito.`)) return;
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
      <div className="bg-white rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-modal animate-slide-up flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100 flex-shrink-0">
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
        <form
          id="product-form"
          onSubmit={handleSubmit}
          className="overflow-y-auto flex-1 p-6"
        >
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
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Foto</label>
            {formData.ID ? (
              <div className="flex flex-wrap items-center gap-2">
                <label className="inline-flex cursor-pointer items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">
                  {uploading ? <FiLoader className="h-4 w-4 animate-spin" /> : <FiCamera className="h-4 w-4" />}
                  {formData.Image ? "Trocar foto" : "Enviar foto"}
                  <input type="file" accept="image/*" className="hidden" disabled={uploading} onChange={handleImage} />
                </label>
                {formData.Image && (
                  <button
                    type="button"
                    className="rounded-xl px-3 py-2 text-sm text-gray-500 hover:bg-gray-50"
                    onClick={() => setFormData((prev) => ({ ...prev, Image: "" }))}
                  >
                    Remover foto
                  </button>
                )}
                <span className="text-xs text-gray-500">Foto do celular pode: reduzimos automaticamente.</span>
              </div>
            ) : (
              <p className="text-xs text-gray-500">Salve o produto primeiro; depois abra de novo para enviar a foto.</p>
            )}
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
              placeholder="Descricao do produto..."
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
                  style={{ background: "#FEF2F2", color: "#DC2626" }}
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
        <div className="flex items-center justify-between px-6 py-4 border-t border-gray-100 bg-gray-50/50 flex-shrink-0">
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
              form="product-form"
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-medium text-white transition-all duration-200 hover:shadow-lg"
              style={{
                background: "linear-gradient(135deg, #DC2626, #B91C1C)",
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
